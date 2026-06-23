package handler

import "github.com/gin-gonic/gin"

type MemoryStoreInterface interface {
	SmartAppend(role, content string) error
	SearchSimilar(query string, topK int) []MemoryRecord
	GetRecent(limit int) []MemoryRecord
	SaveMemoryHandler(c *gin.Context)
	RecallMemoryHandler(c *gin.Context)
	WelcomeHandler(c *gin.Context) // 新增
	CleanMemories()                // 新增
}
