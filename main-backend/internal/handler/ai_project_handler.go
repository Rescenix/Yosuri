package handler

// ai_project_handler.go — AI 项目工坊（写出真实可运行的项目，不是文档）
//   POST /api/ai/project {idea} → LLM 生成完整项目代码 → 落盘 + 编译验证 → 返回
// 输入想法 → 得到可运行项目（Go/Python/JS 多文件，编译通过）

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleAIProject POST /api/ai/project
func HandleAIProject(c *gin.Context) {
	var req struct {
		Idea string `json:"idea" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入项目想法"})
		return
	}
	idea := strings.TrimSpace(req.Idea)

	prompt := fmt.Sprintf(`你是资深全栈工程师。根据这个想法，设计并写出一个完整可运行的小项目：「%s」

输出（严格 JSON，不要任何其他文字）：
{"name":"项目英文名","desc":"一句话描述","files":[{"path":"main.go","lang":"go","code":"完整代码"},{"path":"README.md","lang":"md","code":"使用说明"}]}

要求：
- 2-4 个文件（一个主代码文件 + README）
- 代码完整可运行，Go 或 Python 或 JS 任一
- 代码直接写在 "code" 字段里，不要用 markdown 围栏，转义好引号和换行`, idea)

	content, err := callLocalAggregate(prompt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "模型暂不可用，请稍后重试"})
		return
	}
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var proj struct {
		Name  string `json:"name"`
		Desc  string `json:"desc"`
		Files []struct {
			Path string `json:"path"`
			Lang string `json:"lang"`
			Code string `json:"code"`
		} `json:"files"`
	}
	if json.Unmarshal([]byte(content), &proj) != nil || len(proj.Files) == 0 {
		c.JSON(http.StatusOK, gin.H{"error": "项目生成格式异常，请重试", "raw": content})
		return
	}

	// 落盘到 ~/rescene_data/outputs/projects/<name>/
	home, _ := os.UserHomeDir()
	projDir := filepath.Join(home, "rescene_data", "outputs", "projects", sanitizeProjectName(proj.Name))
	os.MkdirAll(projDir, 0o755)

	type resultFile struct {
		Path     string `json:"path"`
		Lang     string `json:"lang"`
		Code     string `json:"code"`
		Verified string `json:"verified"`
	}
	var results []resultFile
	for _, f := range proj.Files {
		if f.Path == "" || f.Code == "" {
			continue
		}
		// 防路径穿越
		clean := filepath.Base(f.Path)
		os.WriteFile(filepath.Join(projDir, clean), []byte(f.Code), 0o644)
		// 编译验证（主代码文件）
		verified := verifyProjectFile(projDir, clean, f.Lang)
		results = append(results, resultFile{Path: clean, Lang: f.Lang, Code: f.Code, Verified: verified})
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    proj.Name,
		"desc":    proj.Desc,
		"dir":     projDir,
		"files":   results,
		"created": time.Now().Format("2006-01-02 15:04"),
	})
}

// sanitizeProjectName 项目名清洗为安全目录名
func sanitizeProjectName(s string) string {
	repl := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "")
	s = repl.Replace(s)
	if s == "" {
		return "project"
	}
	return s
}

// verifyProjectFile 编译验证单文件（go build / python 语法）
func verifyProjectFile(dir, name, lang string) string {
	full := filepath.Join(dir, name)
	switch lang {
	case "go":
		tmp := filepath.Join(os.TempDir(), "rescene-verify-"+name+".exe")
		defer os.Remove(tmp)
		out, err := exec.Command("go", "build", "-o", tmp, full).CombinedOutput()
		if err != nil {
			return "❌ " + firstLine(string(out))
		}
		return "✅ 编译通过"
	case "python", "py":
		out, err := exec.Command("python", "-m", "py_compile", full).CombinedOutput()
		if err != nil {
			return "❌ " + firstLine(string(out))
		}
		return "✅ 语法通过"
	case "js", "node":
		return "📦 JS（需 node 运行验证）"
	default:
		return "📄 文档"
	}
}

// firstLine 取错误第一行
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return strings.TrimSpace(l)
		}
	}
	return s
}
