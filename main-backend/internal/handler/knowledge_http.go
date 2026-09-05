package handler

// knowledge_http.go —— 外挂知识库的 HTTP 端点（前端「知识库抽屉」用）。
//
// 与 native_knowledge_tools.go 的 agent 工具共用同一个 knowledge 包：
//   - GET  /api/knowledge/list   列出知识库现有文件（含大小/修改时间/片段数）
//   - POST  /api/knowledge/upload 上传文档到知识库（multipart，字段 file）
//   - POST  /api/knowledge/delete 删除某个文档（JSON: {name}）
//
// 上传直接落盘到 knowledge.Dir()，现有检索链路（context_provider 自动注入 +
// knowledge_search/knowledge_list 工具）无需任何改动即可生效。

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"backend/internal/knowledge"
)

// HandleKnowledgeList GET /api/knowledge/list
func HandleKnowledgeList(c *gin.Context) {
	files := knowledge.ListFiles()
	type item struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		ModTime int64  `json:"mtime"`
		Chunks  int    `json:"chunks"`
	}
	out := make([]item, 0, len(files))
	for _, f := range files {
		out = append(out, item{Name: f.Name, Size: f.Size, ModTime: f.ModTime, Chunks: f.Chunks})
	}
	c.JSON(http.StatusOK, gin.H{"files": out})
}

// HandleKnowledgeUpload POST /api/knowledge/upload
func HandleKnowledgeUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件"})
		return
	}
	name := filepath.Base(file.Filename)
	ext := strings.ToLower(filepath.Ext(name))
	if !knowledge.SupportedExts[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 md/markdown/txt/docx/pptx/pdf"})
		return
	}
	dir := knowledge.Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建知识库目录失败"})
		return
	}
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": name})
}

// HandleKnowledgeDelete POST /api/knowledge/delete，body: {"name": "xxx.pdf"}
func HandleKnowledgeDelete(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件名"})
		return
	}
	// 只允许删知识库目录内的文件，禁止路径穿越。
	if strings.ContainsAny(req.Name, "/\\") || req.Name == ".." {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
		return
	}
	dst := filepath.Join(knowledge.Dir(), req.Name)
	if err := os.Remove(dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}