package handler

// git 工作树全量 diff（仿 Claude Code 的 main → working tree 面板）。
//
// 两段式设计：列表端点只返回元数据（路径/状态/增删行数），文件内容按需单独拉，
// 面板打开时秒出清单，展开哪个文件才付哪个文件的传输成本。

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const workingDiffMaxFileBytes = 300 * 1024 // 单文件内容上限，超出不传正文

type workingDiffFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // M/A/D/R/U(untracked)
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Binary    bool   `json:"binary"`
}

func gitOut(args ...string) (string, error) {
	cmd := hiddenCommand("git", args...)
	cmd.Dir = GitRepoRoot
	out, err := cmd.Output()
	return string(out), err
}

// HandleGitWorkingDiff GET /api/git/working-diff — 相对 HEAD 的全部改动文件清单
func HandleGitWorkingDiff(c *gin.Context) {
	branchOut, _ := gitOut("rev-parse", "--abbrev-ref", "HEAD")
	branch := strings.TrimSpace(branchOut)

	// 已跟踪文件的增删行数：git diff HEAD --numstat（暂存+未暂存都覆盖）
	// 二进制文件 numstat 输出 "-\t-\t<path>"
	type numstat struct {
		add, del int
		binary   bool
	}
	stats := map[string]numstat{}
	numstatOut, _ := gitOut("diff", "HEAD", "--numstat")
	for _, line := range strings.Split(strings.TrimSpace(numstatOut), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		path := parts[2]
		// rename 格式 "old => new" 或 "prefix/{old => new}/suffix"，取重命名后的真实路径
		if strings.Contains(path, " => ") {
			path = renamedNewPath(path)
		}
		if parts[0] == "-" {
			stats[path] = numstat{binary: true}
			continue
		}
		add, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		stats[path] = numstat{add: add, del: del}
	}

	// 文件清单来自 git status --porcelain（含未跟踪）。
	// 注意不能对整段输出 TrimSpace：porcelain 首列可能就是空格（" M path"），
	// 整段 Trim 会吃掉第一行的状态位，导致第一个文件的路径少第一个字符
	statusOut, _ := gitOut("status", "--porcelain")
	files := make([]workingDiffFile, 0, 32)
	for _, line := range strings.Split(statusOut, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		if strings.Contains(path, " -> ") { // rename 状态行 "old -> new"
			path = path[strings.Index(path, " -> ")+4:]
		}
		path = strings.Trim(path, `"`) // 中文等路径 git 会加引号

		f := workingDiffFile{Path: path}
		switch {
		case code == "??":
			f.Status = "U"
			// 未跟踪文件：行数即新增数（读文件数行，二进制/超大不数）
			full := filepath.Join(GitRepoRoot, filepath.FromSlash(path))
			if info, err := os.Stat(full); err == nil && !info.IsDir() && info.Size() <= workingDiffMaxFileBytes {
				if data, err := os.ReadFile(full); err == nil {
					if bytes.IndexByte(data, 0) >= 0 {
						f.Binary = true
					} else {
						f.Additions = strings.Count(string(data), "\n") + 1
					}
				}
			}
		case code == "!!":
			continue
		default:
			// 状态码取工作区/暂存区里更有信息量的那个字符
			f.Status = pickStatusChar(code)
			if st, ok := stats[path]; ok {
				f.Additions, f.Deletions, f.Binary = st.add, st.del, st.binary
			}
		}
		files = append(files, f)
	}

	c.JSON(http.StatusOK, gin.H{"branch": branch, "files": files})
}

func pickStatusChar(code string) string {
	for _, ch := range []byte{'D', 'A', 'R', 'C', 'M'} {
		if code[0] == ch || code[1] == ch {
			return string(ch)
		}
	}
	return strings.TrimSpace(code)
}

// renamedNewPath 解析 numstat 的 rename 路径表示，返回新路径。
// "a/{old => new}/b" → "a/new/b"；"old => new" → "new"
func renamedNewPath(p string) string {
	if i := strings.Index(p, "{"); i >= 0 {
		if j := strings.Index(p, "}"); j > i {
			inner := p[i+1 : j]
			if k := strings.Index(inner, " => "); k >= 0 {
				return p[:i] + inner[k+4:] + p[j+1:]
			}
		}
	}
	if k := strings.Index(p, " => "); k >= 0 {
		return p[k+4:]
	}
	return p
}

// HandleGitWorkingDiffFile GET /api/git/working-diff/file?path=... — 单文件新旧内容
func HandleGitWorkingDiffFile(c *gin.Context) {
	rel := strings.TrimSpace(c.Query("path"))
	if rel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 参数必填"})
		return
	}
	// 路径安全：必须是仓库内的相对路径
	if filepath.IsAbs(rel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只接受仓库内相对路径"})
		return
	}
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if strings.HasPrefix(cleaned, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径越界"})
		return
	}

	// 旧内容：HEAD 版本（新文件/未跟踪 → 空）
	oldContent, _ := gitOut("show", "HEAD:"+filepath.ToSlash(cleaned))

	// 新内容：工作区当前版本（已删除 → 空）
	newContent := ""
	tooLarge := false
	full := filepath.Join(GitRepoRoot, cleaned)
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		if info.Size() > workingDiffMaxFileBytes {
			tooLarge = true
		} else if data, err := os.ReadFile(full); err == nil {
			newContent = string(data)
		}
	}

	if len(oldContent) > workingDiffMaxFileBytes {
		tooLarge = true
	}
	binary := bytes.IndexByte([]byte(oldContent), 0) >= 0 || bytes.IndexByte([]byte(newContent), 0) >= 0
	if tooLarge || binary {
		oldContent, newContent = "", ""
	}

	c.JSON(http.StatusOK, gin.H{
		"path": filepath.ToSlash(cleaned), "old_content": oldContent, "new_content": newContent,
		"binary": binary, "too_large": tooLarge,
	})
}
