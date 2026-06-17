package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

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

	// 加载 .env 文件（优先当前目录，次选可执行文件同目录）
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

	}
	// 默认不设，走 handler 包里的 Windows 默认值
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
