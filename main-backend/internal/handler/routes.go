package handler

import (
	"backend/internal/ai/core"

	"backend/internal/middleware"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, memoryStore *MemoryStore, sessionStore *SessionStore) {

	// 全局 CORS 处理（必须在所有路由之前）
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent) // 204
			return
		}
		c.Next()
	})

	chatHandler := NewChatHandler(memoryStore, sessionStore) // 新增
	core.RegisterCleanFunc(func() {
		memoryStore.CleanMemories()
	})

	// r.Use(func(c *gin.Context) {
	// 	c.Header("Access-Control-Allow-Origin", "*")
	// 	c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	// 	c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
	// 	if c.Request.Method == "OPTIONS" {
	// 		c.AbortWithStatus(204)
	// 		return
	// 	}
	// 	c.Next()
	// })
	r.POST("/api/run", RunCodeHandler)
	r.POST("/api/chat/stream", chatHandler.StreamChat) // 新增
	// 以下是你原有的路由，保持不变
	r.PATCH("/api/posts/:id", UpdatePostTags)
	r.DELETE("/api/posts/:id", DeletePost)
	r.GET("/api/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		history := sessionStore.Get(id)
		c.JSON(200, history)
	})

	r.GET("/api/all-messages", func(c *gin.Context) {
		GetAllMessages(c, sessionStore)
	})

	r.DELETE("/api/images/remove", DeleteImage)
	r.POST("/api/upload", UploadToOSS)
	r.GET("/api/images", ListImages)
	r.GET("/api/images/view", ViewImage)
	r.POST("/api/images/tag", UpdateImageTag)
	r.GET("/api/tmp/img/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		c.File("/tmp/shanxi_uploads/" + filename)
	})
	r.GET("/api/admin/clean-memories", func(c *gin.Context) {
		memoryStore.CleanMemories()
		c.JSON(200, gin.H{"status": "ok", "message": "记忆清理已触发，请查看控制台日志"})
	})
	r.GET("/api/balance", GetBalance)
	r.GET("/api/shanxi/status", func(c *gin.Context) {
		hour := time.Now().Hour()
		var status string
		switch {
		case hour >= 0 && hour < 6:
			status = "正在休眠..."
		case hour >= 6 && hour < 9:
			status = "刚刚醒来，正在整理思绪..."
		case hour >= 9 && hour < 18:
			status = "活跃中，随时准备帮忙"
		case hour >= 18 && hour < 22:
			status = "晚间模式，陪你聊聊天"
		default:
			status = "深夜了，但还在线"
		}
		c.JSON(200, gin.H{"status": status})
	})
	r.POST("/api/tts", func(c *gin.Context) {
		var req struct {
			Text string `json:"text" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		audio, err := SynthesizeSpeech(req.Text)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "语音合成失败"})
			return
		}
		c.Data(http.StatusOK, "audio/wav", audio)
	})
	r.GET("/api/images/random", RandomImageWithAI)
	r.DELETE("/api/tags", DeleteTag)
	r.GET("/api/sessions", func(c *gin.Context) {
		sessions := sessionStore.List()
		c.JSON(200, sessions)
	})
	r.POST("/api/sessions", func(c *gin.Context) {
		id := fmt.Sprintf("sess_%d", time.Now().UnixNano())
		c.JSON(200, gin.H{"session_id": id})
	})
	r.POST("/api/chat", func(c *gin.Context) { HandleChat(c, memoryStore, sessionStore) })
	r.GET("/api/posts", GetPosts)
	r.POST("/api/posts", CreatePost)
	r.POST("/api/login", Login)
	r.GET("/api/memory/welcome", memoryStore.WelcomeHandler)

	auth := r.Group("/api/memory").Use(middleware.AuthRequired())
	{
		auth.POST("/save", memoryStore.SaveMemoryHandler)
		auth.GET("/recall", memoryStore.RecallMemoryHandler)
	}

	r.GET("/api/book/list", ListBooks)
	r.GET("/api/book/content", GetBookContent)
	r.POST("/api/book/upload", UploadBook)
	r.DELETE("/api/book/delete", DeleteBook)
	r.GET("/api/admin/clear-redis", func(c *gin.Context) {
		if redisEnabled {
			ctx := context.Background()
			redisClient.FlushAll(ctx)
			c.JSON(200, gin.H{"status": "ok", "message": "Redis 缓存已清空"})
		} else {
			c.JSON(200, gin.H{"status": "disabled", "message": "Redis 未启用"})
		}
	})
	r.POST("/api/image/generate", GenerateImage)
	r.POST("/api/book/upload-cover", UploadCover)
	// 新增：预排版与按页读取

	r.Static("/images", "./public/images")

}
