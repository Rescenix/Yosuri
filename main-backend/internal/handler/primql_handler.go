package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func PrimQLHandler(memoryStore *MemoryStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取请求体"})
			return
		}
		ql := strings.TrimSpace(string(body))
		if ql == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "PrimQL 语句为空"})
			return
		}

		parts := strings.SplitN(ql, " ", 2)
		cmd := strings.ToUpper(parts[0])
		rest := ""
		if len(parts) == 2 {
			rest = strings.TrimSpace(parts[1])
		}

		switch cmd {
		case "LOOM":
			if rest == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "LOOM 需要查询文本"})
				return
			}
			records := memoryStore.SearchSimilar(rest, 20)
			c.JSON(http.StatusOK, records)

		// ENGRAM 写入已禁用，统一由对话流自动保存
		// 如需要调试，可使用 prism_console.html 直接访问 PrismD
		case "ENGRAM":
			c.JSON(http.StatusForbidden, gin.H{"error": "ENGRAM 写入已禁用，记忆由对话流自动保存"})

		case "STATS":
			if memoryStore.prismAddr == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "PrismD 未连接"})
				return
			}
			resp, err := memoryStore.sendPrimQL("STATS")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"raw": resp})

		case "DRIFT":
			if memoryStore.prismAddr == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "PrismD 未连接"})
				return
			}
			_, err := memoryStore.sendPrimQL("DRIFT")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "drift applied"})

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的 PrimQL 命令"})
		}
	}
}
