package handler

// outputs_handler.go — 作品集（outputs/）列表与内容读取 API（发布面板用）
//   GET /api/outputs/list — 列出全部产出文件
//   GET /api/outputs/file?name=xxx.md — 读取文件内容（原始 md）

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func outputsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "daughter", "outputs")
}

// HandleOutputsList GET /api/outputs/list
func HandleOutputsList(c *gin.Context) {
	dir := outputsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"files": []string{}})
		return
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// HandleOutputsFile GET /api/outputs/file?name=xxx.md
func HandleOutputsFile(c *gin.Context) {
	name := c.Query("name")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
		return
	}
	path := filepath.Join(outputsDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在: " + name})
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": name, "content": string(data)})
}