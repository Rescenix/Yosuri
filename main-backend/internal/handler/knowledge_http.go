package handler

// knowledge_http.go —— 外挂知识库的 HTTP 端点（前端「知识库抽屉」用）。
//
// 与 native_knowledge_tools.go 的 agent 工具共用同一个 knowledge 包：
//   - GET  /api/knowledge/list   列出知识库现有文件（含大小/修改时间/片段数）
//   - POST  /api/knowledge/upload 上传文档到知识库（multipart，字段 file）
//   - POST  /api/knowledge/delete 删除某个文档（JSON: {name}）
//   - POST  /api/knowledge/graph  生成知识图谱（LLM 抽取实体/关系，免费模型）
//
// 上传直接落盘到 knowledge.Dir()，现有检索链路（context_provider 自动注入 +
// knowledge_search/knowledge_list 工具）无需任何改动即可生效。

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"backend/internal/knowledge"
)

// HandleKnowledgeList GET /api/knowledge/list?scope=global|session&session_id=xxx
func HandleKnowledgeList(c *gin.Context) {
	dir := knowledge.Dir()
	if c.Query("scope") == "session" {
		dir = knowledge.SessionDir(c.Query("session_id"))
	}
	files := knowledge.ListFilesIn(dir)
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
// multipart：file + scope(global|session) + session_id
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
	if c.PostForm("scope") == "session" {
		dir = knowledge.SessionDir(c.PostForm("session_id"))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建知识库目录失败"})
		return
	}
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	// 清掉旧缓存
	knowledge.InvalidateGraph(dst)
	knowledge.InvalidateAllGraphs()
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": name})
}

// HandleKnowledgeDelete POST /api/knowledge/delete，body: {"name": "xxx.pdf", "scope": "global|session", "session_id": "..."}
func HandleKnowledgeDelete(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		Scope     string `json:"scope"`
		SessionID string `json:"session_id"`
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
	dir := knowledge.Dir()
	if req.Scope == "session" {
		dir = knowledge.SessionDir(req.SessionID)
	}
	dst := filepath.Join(dir, req.Name)
	if err := os.Remove(dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	knowledge.InvalidateGraph(dst)
	knowledge.InvalidateAllGraphs()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// graphReq 知识图谱请求。
type graphReq struct {
	// File 为空则生成全部文件的合并图谱。
	File      string `json:"file,omitempty"`
	Scope     string `json:"scope,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// HandleKnowledgeGraph POST /api/knowledge/graph
// 用免费模型池的 LLM 从知识文档中抽取实体和关系，生成知识图谱。
// 结果按文件 mtime 缓存，只有文件被修改后才重新生成。
func HandleKnowledgeGraph(c *gin.Context) {
	var req graphReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	// LLM 调用函数：用免费模型池，避免消耗用户额度。
	llmCall := func(text string) string {
		msgs := []map[string]any{
			{"role": "system", "content": graphSystemPrompt},
			{"role": "user", "content": graphUserPrompt(text)},
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
		defer cancel()
		content, _, err := routeChatOnce(ctx, freeOnlyBackends(), msgs, nil)
		if err != nil {
			log.Printf("🔴 [知识图谱] LLM 调用失败: %v", err)
			return ""
		}
		return content
	}

	var graph *knowledge.Graph
	var err error

	dir := knowledge.Dir()
	if req.Scope == "session" {
		dir = knowledge.SessionDir(req.SessionID)
	}

	if req.File != "" {
		// 单文件图谱
		if strings.ContainsAny(req.File, "/\\") || req.File == ".." {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
			return
		}
		path := filepath.Join(dir, req.File)
		graph, err = knowledge.GraphForFile(path, llmCall)
	} else {
		// 全库合并图谱（仅全局，会话级不做全库图谱）
		graph, err = knowledge.GraphForAll(llmCall)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成图谱失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, graph)
}

const graphSystemPrompt = `你是一个知识图谱构建专家。从用户提供的文档中抽取实体和关系，输出 JSON 格式的知识图谱。

规则：
1. 只输出 JSON，不要任何解释文字
2. 实体类型包括：concept（概念）、person（人物）、tool（工具）、technology（技术）、process（流程）、principle（原则）、module（模块）、feature（功能）
3. 关系标签用简短的中文或英文，如 "依赖"、"使用"、"包含"、"实现"、"替代"、"相关"
4. count 表示该实体在文中出现的次数估计（1-10）
5. 节点数量控制在 5-20 个，只保留最核心的实体
6. 连线数量控制在 4-30 条

输出格式：
{
  "nodes": [
    {"id": "实体唯一标识", "name": "显示名称", "type": "实体类型", "count": 出现次数}
  ],
  "links": [
    {"source": "源实体id", "target": "目标实体id", "label": "关系标签"}
  ]
}`

func graphUserPrompt(text string) string {
	return fmt.Sprintf("请从以下文档中抽取实体和关系，生成知识图谱：\n\n---\n%s\n---", text)
}
