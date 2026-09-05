package handler

// agent_handler.go —— 多 Agent 角色卡的 HTTP 接口。
//
//	GET    /api/agents              列出全部角色卡（含头像）
//	POST   /api/agents              新建/更新一张（body: {id?, name, persona, icon, color, character}）
//	DELETE /api/agents/:id          删除（连带私有记忆与头像）
//	POST   /api/agents/:id/avatar   存头像（body: {data: "data:image/...;base64,..."}）
//	DELETE /api/agents/:id/avatar   清除头像
//	GET    /api/agents/:id/memory   读该 Agent 的私有记忆索引 + 正文（前端「它的记忆」面板）

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// HandleListAgents GET /api/agents
func HandleListAgents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"agents": ListAgents()})
}

// HandleSaveAgent POST /api/agents
func HandleSaveAgent(c *gin.Context) {
	var body AgentCard
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	saved, err := UpsertAgent(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fillAgentAvatar(&saved)
	c.JSON(http.StatusOK, gin.H{"ok": true, "agent": saved})
}

// HandleDeleteAgent DELETE /api/agents/:id
func HandleDeleteAgent(c *gin.Context) {
	if err := DeleteAgent(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleSaveAgentAvatar POST /api/agents/:id/avatar
func HandleSaveAgentAvatar(c *gin.Context) {
	var req struct {
		Data string `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	if err := SaveAgentAvatar(c.Param("id"), req.Data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleAgentMemory GET /api/agents/:id/memory
// 返回该 Agent 的私有记忆：索引原文 + 全部记忆文件（前端「它的记忆」面板用）。
// 私有记忆文件数量少（每个 Agent 自己写的），全量返回不做召回打分。
func HandleAgentMemory(c *gin.Context) {
	id := memorydir.SanitizeAgentID(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent id 非法"})
		return
	}
	dir := memorydir.AgentMemoryDir(id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"agent_id": id, "files": []any{}})
		return
	}
	type memFile struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	var files []memFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		files = append(files, memFile{Name: strings.TrimSuffix(e.Name(), ".md"), Content: content})
	}
	c.JSON(http.StatusOK, gin.H{"agent_id": id, "files": files})
}
