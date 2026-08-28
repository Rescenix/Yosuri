package handler

import (
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// lanSync 局域网同步服务实例（手机扫码/填 token 内网连接，零云端）。
var lanSync *LanSyncService

// NewAPIRouter 创建 Web 开发服务器与 Wails 桌面壳共用的完整 API 路由。
// 桌面端仍使用真实 loopback HTTP 服务，因为 Wails v2 AssetServer 在 Windows
// 不支持 SSE 响应流和 WebSocket，而 Agent 工作流与预览都依赖这两项能力。
func NewAPIRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Guest-Uid"},
		AllowCredentials: false,
	}))
	sessionStore := NewSessionStore(ChatSessionsDomain)
	RegisterRoutes(r, sessionStore)
	// 局域网同步服务：独立 0.0.0.0 端口 + token 鉴权，只暴露 /lan/ 端点，
	// re0 主服务继续只听 127.0.0.1，零额外暴露面。
	lanSync = NewLanSyncService(sessionStore, os.Getenv("LAN_SYNC_PORT"))
	lanSync.Start()
	return r
}
