package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"backend/internal/ai/core"
	"backend/internal/database"
	"backend/internal/handler"
	"backend/platform/mobile"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	r := gin.Default()
	r.MaxMultipartMemory = 50 << 20 // 50MB

	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		execPath, _ := os.Executable()
		execDir := filepath.Dir(execPath)
		if err2 := godotenv.Load(filepath.Join(execDir, ".env")); err2 != nil {
			log.Println("未找到 .env 文件，将从系统环境变量读取配置")
		}
	}

	database.InitDB()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
	}))

	// ==================== 平台初始化 ====================
if os.Getenv("SHANXI_PLATFORM") == "mobile" {
    handler.SystemPrompt = mobile.SystemPrompt
    handler.ChatTools = mobile.ChatTools
    handler.DeepSeekTransport = mobile.NewDeepSeekTransport()
    handler.UseSemanticMemory = false
    handler.InitLRUMemory(200) // 手机端保留最近 200 条记忆
}else {
		// 电脑端：启动时初始化本地代码索引
		log.Println("🔄 正在初始化本地代码索引...")
		if err := core.InitCodebaseIndex(); err != nil {
			log.Printf("⚠️ 代码索引初始化失败: %v，search_codebase 将不可用", err)
		} else {
			log.Println("✅ 本地代码索引已就绪")
		}
	}
	// =====================================================

	// 初始化记忆存储路径
	memoryPath := os.Getenv("MEMORY_FILE_PATH")
	if memoryPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic("无法获取用户目录: " + err.Error())
		}
		memoryPath = filepath.Join(homeDir, "shanxi_data", "memory.json")
	}

	memoryStore := handler.NewMemoryStore(memoryPath)

	if os.Getenv("DEV_MODE") != "true" {
		// TODO: 在此启用 JWT 认证中间件
	}

	homeDir, _ := os.UserHomeDir()
	sessionPath := filepath.Join(homeDir, "shanxi_data", "sessions.json")
	sessionStore := handler.NewSessionStore(sessionPath)

	go func() {
		for {
			time.Sleep(1 * time.Minute)
			sessionStore.SaveToFile(sessionPath)
		}
	}()

	handler.RegisterRoutes(r, memoryStore, sessionStore)

	r.Run(":8080")
}
