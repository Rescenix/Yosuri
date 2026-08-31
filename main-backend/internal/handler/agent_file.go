package handler

// agent_file.go —— 主 Agent 的文件交付端点。
//
// 复用公司系统的产物交付范式（company_handler.go 的 HandleCompanyFile）：
// Agent 用 write/bash 落盘的可交付文件（md/pdf/pptx/docx/xlsx/html 等），
// 执行层发现后产 artifact(kind:file) 推给前端；前端交付卡片点「预览」时
// 把这个端点作为 URL 送进右侧预览窗口（md 前端自己转 HTML，其余走 raw）。
//
// 根目录是 agent 的主工作目录（core.GetProjectRoot），不是 companyDir。

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"backend/internal/ai/core"
)

// HandleAgentFile GET /api/agent/file?path=...&raw=1
// 同理 company 产物端点：按扩展名分类返回元信息（+文本类回读 content），
// raw=1 时直接 ServeFile 下载/新开。path 相对主工作目录解析，禁止越界。
func HandleAgentFile(c *gin.Context) {
	raw := c.Query("path")
	if raw == "" || filepath.IsAbs(raw) || strings.Contains(raw, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	clean := filepath.Clean(filepath.FromSlash(raw))
	if pathOutsideRoot(clean) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不允许访问工作目录之外的文件"})
		return
	}
	path := filepath.Join(core.GetProjectRoot(), clean)

	info, err := os.Stat(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在: " + raw})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目录不能作为文件交付"})
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if c.Query("raw") == "1" {
		c.Header("Content-Type", contentType)
		c.Header("Content-Disposition", `inline; filename="`+strings.ReplaceAll(filepath.Base(path), `"`, "")+`"`)
		http.ServeFile(c.Writer, c.Request, path)
		return
	}

	kind := "binary"
	switch ext {
	case ".mp4", ".webm", ".mov":
		kind = "video"
	case ".xlsx", ".xls", ".csv", ".tsv":
		kind = "spreadsheet"
	case ".html", ".htm":
		kind = "html"
	case ".pptx":
		kind = "pptx"
	case ".docx":
		kind = "docx"
	case ".pdf":
		kind = "pdf"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		kind = "image"
	case ".md", ".txt", ".json", ".js", ".ts", ".py", ".go", ".java", ".css", ".srt", ".vtt", ".receipt", ".har":
		kind = "text"
	}

	result := gin.H{"name": filepath.Base(path), "kind": kind, "mime": contentType, "size": info.Size()}
	if kind == "text" || kind == "html" {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
			return
		}
		content := string(data)
		if utf8.RuneCountInString(content) > 120000 {
			content = string([]rune(content)[:120000]) + "\n…"
		}
		result["content"] = content
	}
	c.JSON(http.StatusOK, result)
}