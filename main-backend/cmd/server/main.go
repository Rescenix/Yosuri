package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"backend/internal/ai/core"
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
	database.InitPrismDB()

	// 初始化代码搜索索引（保留原有功能）
	log.Println("🔄 正在初始化本地代码索引...")
	if err := core.InitCodebaseIndex(); err != nil {
		log.Printf("⚠️ 代码索引初始化失败: %v，search_codebase 将不可用", err)
	} else {
		log.Println("✅ 本地代码索引已就绪")
	}

	// CORS 配置
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	// 初始化 PrismD 记忆存储
	memoryStore := handler.NewMemoryStore("")
	if err := memoryStore.ConnectPrism("localhost:5666"); err != nil {
		log.Printf("⚠️ PrismD 连接失败: %v，回退到 JSON 存储", err)
		memoryPath := getMemoryPath()
		memoryStore = handler.NewMemoryStore(memoryPath)
	} else {
		log.Println("⚡ PrismD 数字海马体已连接")
	}

	// 初始化会话存储
	homeDir, _ := os.UserHomeDir()
	sessionPath := filepath.Join(homeDir, "shanxi_data", "sessions.json")
	sessionStore := handler.NewSessionStore(sessionPath)

	go func() {
		for {
			time.Sleep(1 * time.Minute)
			sessionStore.SaveToFile(sessionPath)
		}
	}()
	// 自动拉起 DS 浏览器代理
	if err := handler.EnsureDSNodeServer(); err != nil {
		log.Printf("⚠️ DS 代理启动失败: %v，将在首次对话时重试", err)
	}
	handler.RegisterRoutes(r, memoryStore, sessionStore)

	log.Println("🚀 Prism 引擎已启动，监听端口 :8080")
	if err := r.Run(":8080"); err != nil {
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
