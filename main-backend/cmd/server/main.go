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

	// 注册退出清理：主进程收到 SIGINT/SIGTERM 时显式停掉本地 llama-server（如有），
	// 避免子进程变孤儿继续吃内存。本地 llama 不再随后端启动无条件拉起，
	// 改为选中本地识图模型时按需启动（见 EnsureLocalLlamaServer）。
	handler.RegisterLlamaCleanupOnExit()

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
