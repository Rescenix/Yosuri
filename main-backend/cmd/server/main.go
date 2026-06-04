package main

import (
	"backend/internal/database"
	"backend/internal/handler"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	r := gin.Default()
	// 设置最大上传文件大小为 50MB
	r.MaxMultipartMemory = 50 << 20 // 50MB
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		if err2 := godotenv.Load("C:\\Pro2026\\re0\\backend\\.env"); err2 != nil {
			log.Println("未找到 .env 文件，将从系统环境变量读取配置")
		}
	}

	database.InitDB()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:4321"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// 初始化记忆存储路径
	memoryPath := os.Getenv("MEMORY_FILE_PATH")
	if memoryPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic("无法获取用户目录: " + err.Error())
		}
		memoryPath = filepath.Join(homeDir, "shanxi_data", "memory.json")
	}

	// 初始化记忆存储
	memoryStore := handler.NewMemoryStore(memoryPath)

	// 注册路由
	homeDir, _ := os.UserHomeDir()
	sessionPath := filepath.Join(homeDir, "shanxi_data", "sessions.json")
	sessionStore := handler.NewSessionStore(sessionPath)
	// 启动后台协程，每5分钟自动保存一次
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			sessionStore.SaveToFile("/root/shanxi_data/sessions.json")
		}
	}()

	// handler.RegisterRoutes 需要支持传入 sessionStore（或通过全局变量访问）

	// 启动杉汐的记忆自动清理（每天20:00执行）
	//memoryStore.StartMemoryCleaner()
	// 动态获取用户目录，自动适配 Windows/Linux

	handler.RegisterRoutes(r, memoryStore, sessionStore)
	r.Run(":8080")
}
