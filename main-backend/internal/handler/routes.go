package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, sessionStore *SessionStore) {
	// 全局会话存储引用，供 session_search 等工具使用
	globalSessionStore = sessionStore

	// 公司 API 鉴权（2026-08-26）：写操作（评分/花钱/改标签）必须带 API Key。
	// key 从环境变量 COMPANY_API_KEY 读；未设置 = 本地开发模式，放行（兼容桌面版）。
	// 云端部署必须设置 COMPANY_API_KEY，否则外部可随意刷分/花钱。
	r.POST("/api/company/reviews", companyAuthRequired(), HandleCompanyReviewSubmit)
	r.POST("/api/company/finance/spend", companyAuthRequired(), HandleCompanyFinanceSpend)
	r.POST("/api/company/tags", companyAuthRequired(), HandleCompanyAddTag)
	r.DELETE("/api/company/tags/:id", companyAuthRequired(), HandleCompanyDeleteTag)

	// 全局 CORS 处理
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Guest-Uid")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
	r.GET("/api/git-status", gin.WrapH(http.HandlerFunc(GitStatusHandler)))
	// 本地健康诊断快照（doctor skill 用）：key 一律脱敏（sk-...abcd），只返回配置状态与运行健康
	r.GET("/api/diagnose", HandleDiagnose)
	r.GET("/api/git/branches", GitBranches)
	r.GET("/api/git/graph", GitGraph)
	r.POST("/api/git/checkout", GitCheckout)
	r.POST("/api/git/branches", GitCreateBranch)
	// git 工作树全量 diff（Diff 面板）
	r.GET("/api/git/working-diff", HandleGitWorkingDiff)
	r.GET("/api/git/working-diff/file", HandleGitWorkingDiffFile)
	// 主 Agent 文件交付端点：交付卡片预览/下载（仿公司产物端点）
	r.GET("/api/agent/file", HandleAgentFile)
	chatHandler := NewChatHandler(sessionStore)
	r.GET("/api/file-tree", gin.WrapH(http.HandlerFunc(FileTreeHandler)))
	r.GET("/api/file", gin.WrapH(http.HandlerFunc(FileReadHandler)))
	r.POST("/api/file", gin.WrapH(http.HandlerFunc(FileWriteHandler)))
	r.POST("/api/folder", gin.WrapH(http.HandlerFunc(FileCreateFolderHandler)))
	r.POST("/api/file/rename", gin.WrapH(http.HandlerFunc(FileRenameHandler)))
	r.DELETE("/api/file", gin.WrapH(http.HandlerFunc(FileDeleteHandler)))
	r.GET("/api/file/changes", gin.WrapH(http.HandlerFunc(FileChangesHandler)))
	// agent 实际工具执行的工作目录：GET 读当前值，POST 真正切换 + 落盘持久化
	r.GET("/api/workdir", GetWorkdir)
	r.POST("/api/workdir/pick", PickWorkdir)
	r.POST("/api/workdir", SetWorkdir)
	// 受保护工作区：默认关闭；开启后 filesystem MCP 限定项目根，Agent 越界文件访问硬拒绝。
	r.GET("/api/protected-workspace/config", HandleGetProtectedWorkspaceConfig)
	r.PUT("/api/protected-workspace/config", HandlePutProtectedWorkspaceConfig)
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
	// 悬浮球演示面板：只读旁听同一份四态机事件 + 承载页面本身
	r.GET("/api/agent/watch", HandleAgentWatch)
	r.GET("/overlay", HandleOverlayPage)
	r.GET("/overlay/icon.png", HandleOverlayIcon)
	// 悬浮球演示功能开关（测试功能，默认关闭，见 overlay_config.go）
	r.GET("/api/overlay/config", HandleGetOverlayConfig)
	r.PUT("/api/overlay/config", HandlePutOverlayConfig)
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
	// 站点：将当前项目的静态构建发布到用户自己的 Netlify 账号。
	r.GET("/api/sites", HandleSites)
	r.POST("/api/sites/deploy", DeploySite)
	// CDP 截屏中转：前端只连同源 WebSocket，后端连接本机 Chrome 9222
	r.GET("/api/preview/cdp", HandlePreviewCDP)
	// Python Harness (:8001) 集成示例：转发 /run_task
	r.GET("/api/harness/demo", HandleHarnessDemo)
	// 后台任务：前端面板直接查状态/日志/终止（无需经 agent 工具）
	r.GET("/api/bg-task/status", HandleBgTaskStatus)
	r.GET("/api/bg-task/log", HandleBgTaskLog)
	r.POST("/api/bg-task/kill", HandleBgTaskKill)

	// 创作工作台：文案成片（曼波视频一键生成 + 产物静态服务）
	r.POST("/api/studio/mambo", HandleStudioMambo)
	r.POST("/api/studio/manga/plan", HandleMangaPlan)
	r.POST("/api/studio/manga/shot", HandleMangaShot)
	r.GET("/api/studio/files/:file", HandleStudioFiles)
	// 素材库：扫描本地素材目录动态返回（不硬编码）
	r.GET("/api/studio/library", HandleStudioLibrary)
	// 素材库：上传参考素材（图片/视频）/ 删除 / 私密管理
	r.POST("/api/studio/upload", HandleStudioUpload)
	r.DELETE("/api/studio/library/:file", HandleStudioLibraryDelete)
	// 短剧工作台：Agnes 图生视频（异步提交 + 状态查询）
	r.POST("/api/studio/agnes", HandleStudioAgnesSubmit)
	r.POST("/api/studio/agnes/chain", HandleStudioAgnesChain)
	r.GET("/api/studio/agnes/status/:id", HandleStudioAgnesStatus)
	// 素材库：视频抽帧转图片参考
	r.POST("/api/studio/frames", HandleStudioExtractFrame)
	r.POST("/api/translate", HandleTranslate)
	r.POST("/api/translate/batch", HandleTranslateBatch)

	// 设置面板：技能库 / DHS 生态 / 用户档案（含自定义指令）
	r.GET("/api/skills", HandleListSkills)
	r.GET("/api/skills/aggregate", HandleAggregateSkills)
	r.POST("/api/skills/aggregate/sync", HandleSyncAggregateSkill)
	r.POST("/api/skills/:name/status", HandleUpdateSkillStatus)
	r.DELETE("/api/skills/:name", HandleDeleteSkill)
	r.GET("/api/skills/registry", HandleSkillRegistry)
	r.POST("/api/skills/registry/install", HandleInstallHostedSkill)
	r.DELETE("/api/skills/external/:id", HandleDeleteExternalSkill)
	// DHS 社区：发现只读，安装前固定 commit 并通过代码层 + 执行层沙盒审计。
	r.GET("/api/dhs/community", HandleDHSCommunitySearch)
	r.GET("/api/dhs/community/preview", HandleDHSCommunityPreview)
	r.POST("/api/dhs/community/audit", HandleDHSCommunityAudit)
	r.POST("/api/dhs/community/install", HandleDHSCommunityInstall)
	r.POST("/api/dhs/community/install-dhs", HandleDHSCommunityInstallDHS)
	r.POST("/api/dhs/community/uninstall-dhs", HandleDHSCommunityUninstallDHS)
	// DHS 审计账本：云端持久化（读公开，上报带 JWT 透传云端验）
	r.GET("/api/dhs/audits", CloudDHSAuditsProxy)
	r.POST("/api/dhs/audits", CloudDHSAuditsProxy)
	// DHS 爱心收藏：云端按 uid 持久化（登录后跨设备恢复）
	r.GET("/api/dhs/favorites", CloudDHSFavoritesProxy)
	r.POST("/api/dhs/favorites/toggle", CloudDHSFavoritesProxy)
	r.GET("/api/mcp", HandleMCPStatus)
	r.GET("/api/mcp/registry", HandleMCPRegistry)
	r.POST("/api/mcp/registry/install", HandleInstallRegistryMCP)
	r.DELETE("/api/mcp/registry/:name", HandleUninstallRegistryMCP)
	r.GET("/api/profile", HandleGetProfile)
	r.POST("/api/profile", HandleSaveProfile)

	// Aether 视觉预处理（Gemini Interactions REST，纯 net/http，不依赖 SDK）
	r.POST("/api/aether/vision-preprocess", HandleAetherVisionPreprocess)

	// 视频理解：上传视频 → ffmpeg 抽帧 → 视觉模型链（多图 image_url）
	r.POST("/api/video/analyze", HandleVideoAnalyze)

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
	r.PUT("/api/sessions/:id/title", func(c *gin.Context) {
		id := c.Param("id")
		var body struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		sessionStore.SetSessionTitle(id, strings.TrimSpace(body.Title))
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.POST("/api/title/generate", HandleGenerateTitle)

	r.POST("/api/login", CloudLoginProxy)
	r.POST("/api/auth/register", CloudRegisterProxy)
	// token 校验直接代理到 ResceneCloud 云端验签（透传 Authorization 头）。
	// re0 开源侧不持有任何密钥，验签全由云端完成（与签发同源），
	// 既修了「打包版无 .env → 本地默认密钥与云端不符 → auth/me 401」的登不上问题，
	// 又避免了把密钥硬编码进开源代码导致可伪造任意 token 的漏洞。
	r.GET("/api/auth/me", CloudMeProxy)
	// UID 账号体系（薄代理）：由 ResceneCloud 验证并分发，前端不可伪造。
	// 游客持设备指纹换 UID；登录后 bind 把游客号升级为正式账号（永久保留）。
	r.POST("/api/auth/uid", CloudUidProxy)
	r.POST("/api/auth/uid/bind", CloudUidBindProxy)
	// 账号邮箱绑定（老用户注册时没填的补渠道）：登录后发验证码 + 校验绑定，透传 JWT
	r.POST("/api/auth/bind-send-code", CloudBindEmailSendCodeProxy)
	r.POST("/api/auth/bind-email", CloudBindEmailProxy)
	// 游客 JWT 缓存：前端 uid 分发拿到 token 后交给本地后端，供记忆同步鉴权
	r.POST("/api/auth/guest-token", HandleGuestTokenStore)
	// 登录 JWT 缓存：前端登录/refresh 成功后交给本地后端，供记忆同步鉴权
	// （2026-08-30：登录用户必须用登录 token 推记忆，guest token uid 是游客号，
	// 与登录账号 uid 不匹配 → 云端 requireUIDMatch 403 → 零上传）
	r.POST("/api/auth/login-token", HandleLoginTokenStore)
	// 登录 token 恢复：WebView2 localStorage 随 exe 路径丢失时，从 rescene_data 持久化文件恢复登录态
	r.GET("/api/auth/login-token", HandleLoginTokenGet)
	// 登录 token 清除：登出时清掉缓存，避免重启后自动恢复登录态
	r.DELETE("/api/auth/login-token", HandleLoginTokenDelete)
	// 亲密度（无上限互动值）：云端权威存储，re0 透传 + 本地缓存
	r.POST("/api/auth/intimacy/inc", CloudIntimacyIncProxy)
	r.GET("/api/auth/intimacy", CloudIntimacyGetProxy)
	r.GET("/api/evolve/me", HandleEvolveMe)
	r.GET("/api/evolve/events", HandleEvolveEvents)
	// 主页统计云端账本（热力图 + 模型数据，按 uid 累计）：re0 透传
	r.POST("/api/stats/inc", CloudStatsIncProxy)
	r.GET("/api/stats", CloudStatsGetProxy)
	// 云端记忆同步（可选）：记忆包按 uid 存储；pull/push 供前端显式触发
	r.POST("/api/memory/sync/pull", HandleMemorySyncPull)
	r.POST("/api/memory/sync/push", HandleMemorySyncPush)
	// 云端记忆同步开关（前端"记忆"tab 控制；env 可强制关闭）
	r.GET("/api/memory/sync/settings", HandleMemorySyncSettings)
	r.POST("/api/memory/sync/settings", HandleMemorySyncSettingsUpdate)
	// 暴露 ResceneCloud 基址给前端，供其直接发起 GitHub 登录跳转
	r.GET("/api/auth/cloud-config", CloudAuthConfig)
	r.GET("/api/memory/inject", HandleMemoryInject)
	// 新闻标题抓取代理：DS 搜索 open_page 只给 URL，前端要显示标题走这里（防 CORS）
	r.GET("/api/fetch-title", HandleFetchTitle)

	// 自动更新：启动时检查 GitHub 最新 release，有新版则前端弹窗提示更新内容
	r.GET("/api/update/check", HandleCheckUpdate)
	r.GET("/api/update/last-applied", HandleLastAppliedUpdate) // 已更新到 vX 一次性提示（2026-08-18）
	r.POST("/api/update/open", HandleOpenUpdateDownload)
	// 后台自动下载安装包 + 进度 + 拉起安装程序
	r.POST("/api/update/download", HandleAutoDownload)
	r.GET("/api/update/download/status", HandleUpdateDownloadStatus)
	r.POST("/api/update/install", HandleInstallUpdate)
	r.DELETE("/api/update/pending", HandleClearPendingHotPatch) // 跳过此版本：删待应用热补丁 exe

	// 定时任务：前端 ScheduledTaskModal 创建 → 调度器到点弹 Windows 原生通知（右下角）
	r.POST("/api/cron/create", HandleCronCreate)
	r.GET("/api/cron/list", HandleCronList)
	r.DELETE("/api/cron/delete/:id", HandleCronDelete)
	r.POST("/api/cron/test", HandleCronTest)

	// 视觉分析 HTTP 入口，供前端上传/追问复用；Go 内置 view_image 直接复用同一模型路由。
	r.POST("/api/vision/analyze", HandleVisionAnalyze)

	// 用户自定义 API 接入配置（设置面板用，QQ 登录接入前先用固定 "default" 用户）
	r.GET("/api/models/config", HandleGetModelConfig)
	r.PUT("/api/models/config", HandlePutModelConfig)
	r.POST("/api/models/discover", HandleDiscoverProviderModels)
	r.PUT("/api/models/free-order", HandlePutFreeModelOrder)

	// Rescene 聚合 API：OpenAI 兼容端点，聚合免费模型池 + 自定义提供方
	r.POST("/v1/chat/completions", HandleAggregateChat)
	// /v1/responses —— Responses 协议翻译层（2026-08-26）：新版 OpenAI 生态
	// 客户端（Claude Code / Codex / agent 框架）默认优先走 /responses
	r.POST("/v1/responses", HandleAggregateResponses)
	r.GET("/v1/models", HandleAggregateModels)
	// 聚合 API 健康度可视化（设置面板「聚合 API」tab）
	r.GET("/api/aggregate/health", HandleAggregateHealth)
	// 聚合 API 暴露模型配置（official 官方遴选 / custom 用户自定义，issue #5）
	r.GET("/api/aggregate/config", HandleGetAggregateConfig)
	r.PUT("/api/aggregate/config", HandlePutAggregateConfig)
	// 一键同步/还原：导出外部工具配置片段 + 写回（带自动备份）
	r.GET("/api/aggregate/export", HandleAggregateExport)
	r.POST("/api/aggregate/sync", HandleAggregateSync)
	r.POST("/api/aggregate/restore", HandleAggregateRestore)

	// 共享池（公益免费）：代理到 ResceneCloud，云端限流 + 共享 Key 路由
	r.GET("/api/models/shared-pool", HandleGetSharedPoolModels)
	r.POST("/api/chat/shared-pool", HandleSharedPoolChat)
	// 配额查询：本地网关路由前检查公益免费模型配额
	r.GET("/api/shared-pool/quota", HandleSharedPoolQuotaProxy)

	// 云端通知：透传 Authorization 到 ResceneCloud
	r.GET("/api/notifications", CloudNotificationProxy)
	r.POST("/api/notifications/read", CloudNotificationProxy)
	r.POST("/api/notifications/clear", CloudNotificationProxy)
	// 本地通知：不依赖云端，脚本/前端可直接创建（人设周报等）
	r.GET("/api/notifications/local", HandleLocalNotifList)
	r.POST("/api/notifications/local", HandleLocalNotifCreate)
	r.POST("/api/notifications/read/local", HandleLocalNotifRead)

	r.Static("/images", "./public/images")

	// 多平台发布（GUI 发布面板：网文平台一键发布 + Edge CDP cookie 自动化）
	r.GET("/api/publish/platforms", HandlePublishPlatforms)
	r.GET("/api/publish/books", HandleNovelBooks)
	r.POST("/api/publish/books", HandleCreateNovelBook)
	r.PUT("/api/publish/books/:id", HandleUpdateNovelBook)
	r.DELETE("/api/publish/books/:id", HandleDeleteNovelBook)
	r.POST("/api/publish/books/:id/open", HandleOpenNovelBook)
	r.POST("/api/publish/books/:id/chapters", HandleSaveNovelChapter)
	r.POST("/api/publish/compose", HandlePublishCompose)
	r.POST("/api/publish", HandlePublish)
	r.POST("/api/publish/login-edge", HandlePublishLoginEdge)
	// 作品集列表（发布面板选文件用）
	r.GET("/api/outputs/list", HandleOutputsList)
	r.GET("/api/outputs/file", HandleOutputsFile)
	// 公司管理面板（多 Agent 编排 GUI）
	r.GET("/api/company/agents", HandleCompanyAgents)
	r.GET("/api/company/agent", HandleCompanyAgent)
	r.GET("/api/company/file", HandleCompanyFile)
	r.GET("/api/company/integrations", HandleCompanyIntegrations)
	r.GET("/api/company/production-audit", HandleCompanyProductionAudit)
	r.GET("/api/company/evolve", HandleCompanyEvolve)
	r.GET("/api/company/goals", HandleCompanyGoals)
	r.POST("/api/company/goals", HandleCreateCompanyGoal)
	r.GET("/api/company/goals/:id", HandleCompanyGoal)
	r.POST("/api/company/goals/:id/run", HandleRunCompanyGoal)
	r.POST("/api/company/goals/:id/decision", HandleCompanyGoalDecision)
	r.GET("/api/company/goals/:id/artifacts/:artifactId", HandleCompanyArtifact)
	r.GET("/api/company/os-stats", HandleCompanyOSStats)
	// 审批工作台（用户只审批，不聊天）
	r.GET("/api/company/approvals", HandleCompanyApprovals)
	r.POST("/api/company/approve", HandleCompanyApprove)
	r.GET("/api/company/approval-history", HandleCompanyApprovalHistory)
	r.GET("/api/company/meetings", HandleCompanyMeetings)
	// 公司标签系统（调研方向 + 热门标签云端维护）
	r.GET("/api/company/tags", HandleCompanyTags)
	// 迭代计划（预设：从已审批项目选，每天调研前沿技术迭代产品）
	r.GET("/api/company/iterate", HandleCompanyIterate)
	r.POST("/api/company/iterate", HandleCompanyIterateStart)
	r.POST("/api/company/iterate/stop", HandleCompanyIterateStop)
	r.GET("/api/company/tags/hot", HandleCompanyHotTags)
	// 用户自定义指令（考题/项目目标，立项最高优先级）
	r.GET("/api/company/directive", HandleCompanyDirective)
	r.PUT("/api/company/directive", HandleCompanySaveDirective)
	// 发行评测：真实用户评分评奖（每一个评分都来自真人）
	r.GET("/api/company/reviews", HandleCompanyReviews)
	r.GET("/api/company/finance", HandleCompanyFinance)
	// 应用大厅：B站式信息流（推荐分+推广位）
	r.GET("/api/company/market", HandleCompanyMarket)
	// AI PPT 生成（看得见的幻灯片）
	r.POST("/api/ai/ppt", HandleAIPPT)
	// AI 项目工坊（写出真实可运行项目）
	r.POST("/api/ai/project", HandleAIProject)
	// 漫画创作 Agent（宇宙第一画面Agent）
	r.GET("/api/comic/status", HandleComicStatus)
	r.POST("/api/comic/start-sd", HandleComicStartSD)
	r.POST("/api/comic/breakdown", HandleComicBreakdown)
	r.POST("/api/comic/generate", HandleComicGenerate)
	r.POST("/api/comic/render-panel", HandleComicRenderPanel)
	r.POST("/api/comic/assemble", HandleComicAssemble)
	r.GET("/api/comic/characters", HandleComicCharacters)
	r.POST("/api/comic/characters", HandleComicCreateCharacter)
	r.GET("/api/comic/pages", HandleComicList)
	r.GET("/api/comic/page/:id", HandleComicPage)
	// 漫画图片静态服务
	r.Static("/api/comic/image", comicOutputDir())
	// 免费无 key 生图（Pollinations 直连，SD 在线时兜底）
	r.POST("/api/image/generate", HandleImageGenerate)
	r.Static("/api/image/file", imageOutputDir())
	// AI 生视频（Agnes 免费 API，$0/秒）产物静态服务
	r.Static("/api/video/file", videoOutputDir())
	// Galgame 模式：AI 生立绘 + 剧本对话 + 选项分支
	r.POST("/api/galgame/new", HandleGalgameNew)
	r.POST("/api/galgame/advance", HandleGalgameAdvance)
	r.POST("/api/galgame/rollback", HandleGalgameRollback)
	r.POST("/api/galgame/portrait", HandleGalgamePortrait)
	r.POST("/api/galgame/background", HandleGalgameBackground)
	r.GET("/api/galgame/sessions", HandleGalgameSessions)
	r.GET("/api/galgame/session/:id", HandleGalgameSession)
	r.DELETE("/api/galgame/session/:id", HandleGalgameDeleteSession)
	// Galgame 立绘/背景静态服务
	r.Static("/api/galgame/asset", galgameOutputDir())
	// 局域网同步信息（前端开关/连接面板展示用）：默认关闭，开启后才监听 0.0.0.0
	r.GET("/api/lan/sync-info", func(c *gin.Context) {
		if lanSync == nil {
			c.JSON(500, gin.H{"error": "局域网同步服务不可用"})
			return
		}
		resp := gin.H{"enabled": lanSync.Running()}
		if lanSync.Running() {
			resp["ip"] = getLanIP()
			resp["port"] = lanSync.Port()
			resp["token"] = lanSync.Token()
			resp["device"] = "re0"
		}
		c.JSON(200, resp)
	})
	// 开启局域网同步（幂等）：开始监听 0.0.0.0:port。首次开启时 Windows 防火墙会弹窗确认
	r.POST("/api/lan/enable", func(c *gin.Context) {
		if lanSync == nil {
			c.JSON(500, gin.H{"error": "局域网同步服务不可用"})
			return
		}
		lanSync.Start()
		c.JSON(200, gin.H{"enabled": true, "ip": getLanIP(), "port": lanSync.Port(), "token": lanSync.Token(), "device": "re0"})
	})
	// 关闭局域网同步（幂等）
	r.POST("/api/lan/disable", func(c *gin.Context) {
		if lanSync != nil {
			lanSync.Stop()
		}
		c.JSON(200, gin.H{"enabled": false})
	})
}
