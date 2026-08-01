package main

import (
	"log"
	"os"

	"backend/internal/handler"

	"github.com/joho/godotenv"
)

func main() {
	// 加载环境变量
	_ = godotenv.Load()

	// 注册退出清理：主进程收到 SIGINT/SIGTERM 时显式停掉预览 Chromium/Edge（如有），
	// 避免子进程变孤儿继续占内存。（本地 llama-server 已移除，2026-08-01。）
	handler.RegisterCleanupOnExit()

	r := handler.NewAPIRouter()

	log.Println("🚀 Rescene 引擎已启动，监听端口 :8080")
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	if err := r.Run(":" + addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
