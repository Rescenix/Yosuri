package main

import (
	"log"
	"os"

	"backend/internal/handler"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 加载环境变量
	_ = godotenv.Load()

	// CORS 配置
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	// 初始化会话存储：按用途分域落本地 JSON 文件（chat_sessions / code_sessions 各自独立），
	// 每次 Append/SetCompressIndex 都会异步整份重写
	sessionStore := handler.NewSessionStore(handler.ChatSessionsDomain)

	// 注册退出清理：主进程收到 SIGINT/SIGTERM 时显式停掉本地 llama-server（如有），
	// 避免子进程变孤儿继续吃内存。本地 llama 不再随后端启动无条件拉起，
	// 改为选中本地识图模型时按需启动（见 EnsureLocalLlamaServer）。
	handler.RegisterLlamaCleanupOnExit()

	handler.RegisterRoutes(r, sessionStore)

	log.Println("🚀 Rescene 引擎已启动，监听端口 :8080")
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	if err := r.Run(":" + addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
