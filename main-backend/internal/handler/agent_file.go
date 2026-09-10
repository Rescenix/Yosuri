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

// deliverablePathAllowed 判定一个交付卡片路径能否经本公开读口 serve。
//
// 闸门只跟「受保护工作区」开关挂钩（09-10 定稿）：卡片路径是服务端自己生成推给
// 前端的，不是用户输入，普通模式没有理由拒——之前拿「用户批准过」当放行条件，
// 等于自己弹卡自己拒（音频/视频/记忆文件交付卡必 400 的根因）。
// 唯一无条件保留的是凭据文件黑名单：user_configs 落盘是明文 key，掩码只在 UI 出口，
// 拖文件等于拖明文。这是确定性规则，不是概率审计。
func deliverablePathAllowed(path string) bool {
	if isCredentialFile(path) {
		return false
	}
	if !ProtectedWorkspaceEnabled() {
		return true
	}
	return isApprovedOutsidePath(path)
}

// isCredentialFile 密钥/凭据落盘文件，任何模式都不经公开读口外发。
func isCredentialFile(path string) bool {
	clean := normCase(filepath.Clean(path))
	dir := normCase(resceneUserDataDir())
	rel, err := filepath.Rel(dir, clean)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	// user_configs/ 下全部（含 openid.json），以及数据根里其它明显的凭证文件
	return strings.HasPrefix(rel, normCase("user_configs")+string(filepath.Separator)) ||
		filepath.Base(clean) == "credentials.json"
}

// HandleAgentFile GET /api/agent/file?path=...&raw=1
// 同理 company 产物端点：按扩展名分类返回元信息（+文本类回读 content），
// raw=1 时直接 ServeFile 下载/新开。path 相对主工作目录解析，禁止越界。
func HandleAgentFile(c *gin.Context) {
	raw := c.Query("path")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var path string
	// 绝对路径：交付卡片给的就是这种形态（媒体目录/记忆目录都在工作目录外），
	// 闸门见 deliverablePathAllowed——普通模式全放行，受保护工作区模式才要批准过。
	if filepath.IsAbs(raw) {
		if !deliverablePathAllowed(raw) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该文件不在 Agent 工作目录内，且未经你批准，无法预览/下载"})
			return
		}
		path = filepath.Clean(raw)
	} else {
		if strings.Contains(raw, "..") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		clean := filepath.Clean(filepath.FromSlash(raw))
		if pathOutsideRoot(clean) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该文件不在 Agent 工作目录内，无法预览/下载"})
			return
		}
		path = filepath.Join(core.GetProjectRoot(), clean)
	}

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
