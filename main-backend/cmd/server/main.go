package main

import (
	"log"
	"os"
	"path/filepath"

	"backend/internal/database"
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

	// 初始化数据库连接
	database.InitDB()

	// CORS 配置
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	// 初始化记忆存储（本地 JSON 落盘；实际记忆检索走 SwiftNet，见 internal/swiftnet）
	memoryStore := handler.NewMemoryStore(getMemoryPath())

	// 初始化会话存储：按用途分域落本地 JSON 文件（chat_sessions / code_sessions 各自独立），
	// 每次 Append/SetCompressIndex 都会异步整份重写
	sessionStore := handler.NewSessionStore(handler.ChatSessionsDomain)

	// DS 浏览器代理（crack/server.js）已随 crack/ 目录封存废弃：主链路走 /api/code/workflow，
	// 不再需要 localhost:3000 代理，故不再自动拉起——避免 crack 删除后启动被 node 缺失阻塞。
	handler.RegisterRoutes(r, memoryStore, sessionStore)

	log.Println("🚀 Aurora 引擎已启动，监听端口 :8080")
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	if err := r.Run(":" + addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func getMemoryPath() string {
	memoryPath := os.Getenv("MEMORY_FILE_PATH")
	if memoryPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic("无法获取用户目录: " + err.Error())
		}
		memoryPath = filepath.Join(homeDir, "shanxi_data", "memory.json")
	}
	return memoryPath
}
