package handler

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type GitStatus struct {
	Branch    string   `json:"branch"`
	Modified  []string `json:"modified"`
	Untracked []string `json:"untracked"`
	Added     int      `json:"added"`
	Removed   int      `json:"removed"`
}

var shortstatInsRe = regexp.MustCompile(`(\d+) insertion`)
var shortstatDelRe = regexp.MustCompile(`(\d+) deletion`)

// PR 面板顶部的 +N/-N 用的就是这个：相对 HEAD 的合计增删行数（覆盖已暂存 + 未暂存的
// 已跟踪文件改动），未跟踪的新文件不计入——跟 git diff 本身的语义保持一致
func gitDiffShortstat() (added int, removed int) {
	cmd := exec.Command("git", "diff", "HEAD", "--shortstat")
	cmd.Dir = GitRepoRoot
	out, _ := cmd.Output()
	s := string(out)
	if m := shortstatInsRe.FindStringSubmatch(s); m != nil {
		added, _ = strconv.Atoi(m[1])
	}
	if m := shortstatDelRe.FindStringSubmatch(s); m != nil {
		removed, _ = strconv.Atoi(m[1])
	}
	return
}

func GitStatusHandler(w http.ResponseWriter, r *http.Request) {
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = GitRepoRoot
	branchBytes, _ := branchCmd.Output()
	branch := strings.TrimSpace(string(branchBytes))

	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = GitRepoRoot
	out, err := statusCmd.Output()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var modified, untracked []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		file := strings.TrimSpace(line[3:])

		// 未跟踪
		if status == "??" || status == "!!" {
			untracked = append(untracked, file)
			continue
		}
		// 已修改：暂存区或工作区有 M/A/D/R/C
		for _, c := range status {
			if c == 'M' || c == 'A' || c == 'D' || c == 'R' || c == 'C' {
				modified = append(modified, file)
				break
			}
		}
	}

	added, removed := gitDiffShortstat()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GitStatus{
		Branch:    branch,
		Modified:  modified,
		Untracked: untracked,
		Added:     added,
		Removed:   removed,
	})
}

func GitDiffHandler(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	out, _ := exec.Command("git", "diff", file).Output()
	json.NewEncoder(w).Encode(map[string]string{"diff": string(out)})
}

func GitStageAllHandler(w http.ResponseWriter, r *http.Request) {
	exec.Command("git", "add", "-A").Run()
	w.WriteHeader(200)
}
func GitAddAll(c *gin.Context) {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = GitRepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		c.String(http.StatusInternalServerError, "Add 失败:\n"+string(out))
		return
	}
	c.Status(http.StatusOK)
}

func GitCommit(c *gin.Context) {
	var body struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Message == "" {
		c.String(http.StatusBadRequest, "message required")
		return
	}

	// 使用 git commit -F - 从标准输入读取，完美支持多行
	cmd := exec.Command("git", "commit", "-F", "-")
	cmd.Dir = GitRepoRoot
	cmd.Stdin = strings.NewReader(body.Message)
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.String(http.StatusInternalServerError, "Commit 失败:\n"+string(out))
		return
	}
	c.Status(http.StatusOK)
}

func GitPush(c *gin.Context) {
	cmd := exec.Command("git", "push", "origin", "main")
	cmd.Dir = GitRepoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.String(http.StatusInternalServerError, "Push 失败:\n"+string(out))
		return
	}
	c.String(http.StatusOK, "Push 成功:\n"+string(out))
}
