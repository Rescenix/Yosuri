package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, memoryStore *MemoryStore, sessionStore *SessionStore) {
	r.POST("/api/prismql", PrimQLHandler(memoryStore))
	// 全局 CORS 处理
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	r.GET("/api/git-status", gin.WrapH(http.HandlerFunc(GitStatusHandler)))
	chatHandler := NewChatHandler(memoryStore, sessionStore)
	go chatHandler.warmUpLocalModel() // 显式异步，不阻塞注册
	core.RegisterCleanFunc(func() {
		memoryStore.CleanMemories()
	})

	r.GET("/api/file-tree", gin.WrapH(http.HandlerFunc(FileTreeHandler)))
	r.GET("/api/file", gin.WrapH(http.HandlerFunc(FileReadHandler)))
	r.POST("/api/git/add-all", GitAddAll)
	r.POST("/api/git/commit", GitCommit)
	r.POST("/api/git/push", GitPush)
	r.POST("/api/tool/execute", HandleToolExecute)
	// 根据平台注册不同的聊天处理器
	r.POST("/api/execute-marker", HandleExecuteMarker)
	r.POST("/api/chat/stream", chatHandler.StreamChat)

	// Agent 工作流编排
	workflowRunner := NewWorkflowRunner(chatHandler)
	r.POST("/api/workflow/run", workflowRunner.HandleWorkflowRun)
	r.GET("/api/workflows", workflowRunner.HandleListWorkflows)

	// Aether 视觉预处理（Gemini Interactions REST，纯 net/http，不依赖 SDK）
	r.POST("/api/aether/vision-preprocess", HandleAetherVisionPreprocess)

	r.PATCH("/api/posts/:id", UpdatePostTags)
	r.DELETE("/api/posts/:id", DeletePost)
	r.GET("/api/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		history := sessionStore.Get(id)
		c.JSON(200, history)
	})
	// 会话 v2（基于 PrismD Session 簇）
	r.POST("/api/v2/sessions", CreateSession)
	r.GET("/api/v2/sessions", ListSessions)
	r.PATCH("/api/v2/sessions/:id", RenameSession)
	r.DELETE("/api/v2/sessions/:id", DeleteSession)
	r.GET("/api/all-messages", GetAllMessagesHandler(sessionStore))

	// 统计仪表盘（数据完全来自 SessionStore，不依赖 PrismD）
	statsHandler := NewStatsHandler(sessionStore)
	r.GET("/api/stats/overview", statsHandler.HandleOverview)
	r.GET("/api/stats/daily", statsHandler.HandleDailyStats)
	r.GET("/api/stats/detail", statsHandler.HandleDayDetail)

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

	r.GET("/api/posts", GetPosts)
	r.POST("/api/posts", CreatePost)
	r.POST("/api/login", Login)
	r.GET("/api/memory/welcome", memoryStore.WelcomeHandler)

	r.POST("/api/memory/save", memoryStore.SaveMemoryHandler)
	r.GET("/api/memory/recall", memoryStore.RecallMemoryHandler)

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

	// 用户自定义 API 接入配置（设置面板用，QQ 登录接入前先用固定 "default" 用户）
	r.GET("/api/models/config", HandleGetModelConfig)
	r.PUT("/api/models/config", HandlePutModelConfig)

	r.Static("/images", "./public/images")
}
