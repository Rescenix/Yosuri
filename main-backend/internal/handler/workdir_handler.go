package handler

import (
	"net/http"
	"path/filepath"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

// GetWorkdir GET /api/workdir —— 前端"添加工作目录"面板挂载时用这个同步真实值，
// 不能只信 localStorage：那只是 UI 展示层的缓存，agent 实际用的是后端 core.GetProjectRoot()。
func GetWorkdir(c *gin.Context) {
	root := core.GetProjectRoot()
	c.JSON(http.StatusOK, gin.H{
		"path": root,
		"name": filepath.Base(root),
	})
}

// SetWorkdir POST /api/workdir {"path": "main-frontend"} —— 真正切换 agent 的工作目录。
// path 支持相对路径（相对 GitRepoRoot，跟 /api/file-tree 返回的 path 字段对齐）和绝对路径。
// 切换后 read_file/write_file/edit_file/execute_command/search_codebase 全部立刻生效，
// 并落盘持久化到 ~/shanxi_data/workdir.txt，下次启动自动恢复。
func SetWorkdir(c *gin.Context) {
	var body struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}

	target := body.Path
	if !filepath.IsAbs(target) {
		target = filepath.Join(GitRepoRoot, target)
	}
	target = filepath.Clean(target)

	if err := core.SetProjectRoot(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path": target,
		"name": filepath.Base(target),
	})
}
