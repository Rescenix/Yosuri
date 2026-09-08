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

	// 免费池自动发现预热：后台拉各提供方 /v1/models，一个 key 出全部模型
	// （step 等厂商配过 key 就自动全量进下拉，2026-08-04）。
	handler.WarmFreePoolDiscovery()

	// ResceneCloud 预热：应用启动即后台打一发 /healthz，把 Render 免费实例的冷启动
	// 开销提前到启动阶段，减少用户点「登录」时撞上冷启动超时（2026-08-20）。
	handler.WarmCloudAuth()

	// 自定义语音：加载上一次的云端配置（未配=回落到 Edge 直连）
	handler.LoadTTSConfigFile()

	r := handler.NewAPIRouter()

		// 云端记忆同步：启动后自动拉取一次（换设备恢复记忆）+ 定时双向循环
		// （2026-08-30：cmd/server 之前漏掉这两个调用，纯后端模式永远不同步记忆）
		handler.StartupMemorySyncPull()
		handler.StartMemorySyncLoop()

	log.Println("🚀 Rescene 引擎已启动")
	// 本地默认只听回环：这套 API 能起终端进程、读写工作区文件，
	// 绑 0.0.0.0 等于让同 WiFi 下任何设备直接驱动它。局域网同步是
	// 另一套独立服务（0.0.0.0 + token 鉴权，只暴露 /lan/），不依赖这里。
	// 云端 PaaS 注入 PORT 时保持绑全网卡，否则平台流量进不来。
	// RE0_BIND 可显式覆盖（测试或特殊部署用）。
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	bind := "127.0.0.1"
	if os.Getenv("PORT") != "" {
		bind = ""
	}
	if v, ok := os.LookupEnv("RE0_BIND"); ok {
		bind = v
	}
	if err := r.Run(bind + ":" + port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
