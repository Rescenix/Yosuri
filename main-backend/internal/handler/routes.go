package handler

import (
	"fmt"
	"net/http"
	"time"

	"backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, sessionStore *SessionStore) {
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
	r.GET("/api/git/branches", GitBranches)
	r.GET("/api/git/graph", GitGraph)
	r.POST("/api/git/checkout", GitCheckout)
	r.POST("/api/git/branches", GitCreateBranch)
	// git 工作树全量 diff（Diff 面板）
	r.GET("/api/git/working-diff", HandleGitWorkingDiff)
	r.GET("/api/git/working-diff/file", HandleGitWorkingDiffFile)
	chatHandler := NewChatHandler(sessionStore)
	r.GET("/api/file-tree", gin.WrapH(http.HandlerFunc(FileTreeHandler)))
	r.GET("/api/file", gin.WrapH(http.HandlerFunc(FileReadHandler)))
	r.POST("/api/file", gin.WrapH(http.HandlerFunc(FileWriteHandler)))
	r.GET("/api/file/changes", gin.WrapH(http.HandlerFunc(FileChangesHandler)))
	// agent 实际工具执行的工作目录：GET 读当前值，POST 真正切换 + 落盘持久化
	r.GET("/api/workdir", GetWorkdir)
	r.POST("/api/workdir/pick", PickWorkdir)
	r.POST("/api/workdir", SetWorkdir)
	// AgentFS：本地文件历史时间线（VS Code Timeline 风格，无 git）
	r.POST("/api/agentfs/open", AgentFSOpen)
	r.GET("/api/agentfs/log", AgentFSLog)
	r.POST("/api/agentfs/diff", AgentFSDiff)
	r.POST("/api/agentfs/restore", AgentFSRestore)
	// 真实交互式终端：SSE 输出 + POST 写 stdin，会话按 id 常驻（详见 terminal_handler.go）
	r.GET("/api/terminal/stream", HandleTerminalStream)
	r.POST("/api/terminal/input", HandleTerminalInput)
	r.POST("/api/terminal/close", HandleTerminalClose)
	r.POST("/api/git/add-all", GitAddAll)
	r.POST("/api/git/commit", GitCommit)
	r.POST("/api/git/push", GitPush)

	// Agent 工作流编排
	workflowRunner := NewWorkflowRunner(chatHandler)
	// 四态机 Code 工作流（思考/意图/操作/结果，EventSource 直连）
	r.GET("/api/code/workflow", workflowRunner.HandleCodeWorkflow)
	// 主动停止：先通知后端取消并落盘部分上下文，再由前端关闭 EventSource。
	r.POST("/api/code/workflow/stop", workflowRunner.HandleCodeWorkflowStop)
	// 工具审批回调：Ask 模式下前端批准条「允许/拒绝」写回，恢复四态机执行
	r.POST("/api/code/workflow/approve", workflowRunner.HandleCodeWorkflowApprove)
	// 中途插话：工作流跑着的时候插一条消息，下一轮当作用户中途发言拼进上下文
	r.POST("/api/code/workflow/steer", HandleCodeWorkflowSteer)
	// ask_user 提问回答回调：前端提问弹窗「确认」写回，唤醒阻塞中的 ask_user 续跑
	r.POST("/api/code/workflow/answer", workflowRunner.HandleCodeWorkflowAnswer)
	// 断点续跑：列出中断的工作流；续跑本身走 GET /api/code/workflow?resume=<workflow_id>
	r.GET("/api/code/workflow/checkpoints", workflowRunner.HandleCodeWorkflowCheckpoints)
	r.DELETE("/api/code/workflow/checkpoints/:id", workflowRunner.HandleCodeWorkflowCheckpointDelete)
	// 预览浏览器：本地开发服务器探测
	r.GET("/api/preview/servers", HandlePreviewServers)
	// CDP 截屏中转：前端只连同源 WebSocket，后端连接本机 Chrome 9222
	r.GET("/api/preview/cdp", HandlePreviewCDP)
	// Python Harness (:8001) 集成示例：转发 /run_task
	r.GET("/api/harness/demo", HandleHarnessDemo)

	// 设置面板：技能库 / MCP 生态 / 用户档案（含自定义指令）
	r.GET("/api/skills", HandleListSkills)
	r.POST("/api/skills/:name/status", HandleUpdateSkillStatus)
	r.DELETE("/api/skills/:name", HandleDeleteSkill)
	r.GET("/api/skills/registry", HandleSkillRegistry)
	r.POST("/api/skills/registry/install", HandleInstallHostedSkill)
	r.DELETE("/api/skills/external/:id", HandleDeleteExternalSkill)
	r.GET("/api/mcp", HandleMCPStatus)
	r.GET("/api/mcp/registry", HandleMCPRegistry)
	r.POST("/api/mcp/registry/install", HandleInstallRegistryMCP)
	r.DELETE("/api/mcp/registry/:name", HandleUninstallRegistryMCP)
	r.GET("/api/profile", HandleGetProfile)
	r.POST("/api/profile", HandleSaveProfile)

	// Aether 视觉预处理（Gemini Interactions REST，纯 net/http，不依赖 SDK）
	r.POST("/api/aether/vision-preprocess", HandleAetherVisionPreprocess)

	r.GET("/api/sessions/:id", func(c *gin.Context) {
		id := c.Param("id")
		// 返回持久化视图而不是 []DSMessage：DSMessage 的 Timestamp/Blocks 打了 json:"-"
		// （避免混进发给上游的请求体），直接序列化会把工作流轨迹和时间戳丢干净，
		// 前端就只剩纯文本，刷新后工具调用全没了。
		c.JSON(200, toPersistedMessages(sessionStore.Get(id)))
	})
	r.DELETE("/api/sessions/:id", func(c *gin.Context) {
		sessionStore.Delete(c.Param("id"))
		c.JSON(200, gin.H{"status": "ok"})
	})
	// 编辑并重发历史消息：从该消息之前的位置分叉出一条新分支（keep=拷贝的消息条数），
	// 前端切到新分支再重发。原会话完整保留——以前这里是 truncate，会把后面的对话永久砍掉。
	r.POST("/api/sessions/:id/fork", func(c *gin.Context) {
		var body struct {
			Keep int `json:"keep"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		parentID := c.Param("id")
		newID, ok := sessionStore.Fork(parentID, body.Keep)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在或为空"})
			return
		}
		// 回实际生效的分岐点而不是请求里的 keep —— Fork 会做钳制，两者可能不等
		c.JSON(200, gin.H{"session_id": newID, "parent_id": parentID, "fork_index": sessionStore.ForkIndex(newID)})
	})
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

	r.POST("/api/login", CloudLoginProxy)
	// GitHub OAuth 登录：流量转发到私有鉴权服务 ResceneCloud（/api/auth/github 发起，
	// /api/auth/github/callback 接收回调并带回 JWT）。re0 不持有 OAuth 密钥。
	r.GET("/api/auth/github", CloudGitHubLogin)
	r.GET("/api/auth/github/callback", CloudGitHubCallback)
	// 校验当前 token 真伪（复用 middleware.AuthRequired 本地验 JWT_SECRET）：
	// 仅当 token 有效时返回 200，否则 401。前端用它区分“真登录”与“伪造/过期 token”。
	r.GET("/api/auth/me", middleware.AuthRequired(), AuthMe)
	// 暴露 ResceneCloud 基址给前端，供其直接发起 GitHub 登录跳转
	r.GET("/api/auth/cloud-config", CloudAuthConfig)
	r.GET("/api/memory/inject", HandleMemoryInject)

	// 视觉分析 HTTP 入口，供前端上传/追问复用；Go 内置 view_image 直接复用同一模型路由。
	r.POST("/api/vision/analyze", HandleVisionAnalyze)

	// 用户自定义 API 接入配置（设置面板用，QQ 登录接入前先用固定 "default" 用户）
	r.GET("/api/models/config", HandleGetModelConfig)
	r.PUT("/api/models/config", HandlePutModelConfig)
	r.POST("/api/models/discover", HandleDiscoverProviderModels)

	r.Static("/images", "./public/images")
}
