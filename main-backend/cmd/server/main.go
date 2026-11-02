package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

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
	// 端口：PaaS 注入 PORT 时严格用它（平台流量进不来就废了）。
	// 本地开发从 8080 起，被占就顺延到第一个空闲端口——8080 是 Steam /
	// Docker / Tomcat 的重灾区，桌面版早就这么回退了（desktop_app.go:36），
	// 这里补齐，免得 go run 直接被占用打死。
	port := os.Getenv("PORT")
	bind := "127.0.0.1"
	if port != "" {
		bind = ""
	}
	if v, ok := os.LookupEnv("RE0_BIND"); ok {
		bind = v
	}

	ln, actualPort, err := listenWithFallback(bind, port)
	if err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
	if port == "" && actualPort != "8080" {
		log.Printf("⚠️ 端口 8080 已被占用（常见于 Steam/Docker），已自动改用 %s", actualPort)
	}
	// 只在本机回环模式写端口提示文件，给 vite dev 的 proxy 读
	if bind == "127.0.0.1" {
		writePortHint(actualPort)
	}
	if err := r.RunListener(ln); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// listenWithFallback 端口选择：显式 PORT 必须成功（PaaS）；本地则 8080→8089 顺延，
// 全占满再让系统分配随机端口。
func listenWithFallback(bind, port string) (net.Listener, string, error) {
	if port != "" {
		ln, err := net.Listen("tcp", bind+":"+port)
		if err != nil {
			return nil, "", fmt.Errorf("端口 %s 无法监听: %w", port, err)
		}
		return ln, port, nil
	}
	for p := 8080; p < 8090; p++ {
		addr := bind + ":" + strconv.Itoa(p)
		if ln, err := net.Listen("tcp", addr); err == nil {
			return ln, strconv.Itoa(p), nil
		}
	}
	ln, err := net.Listen("tcp", bind+":0")
	if err != nil {
		return nil, "", err
	}
	_, p, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return nil, "", err
	}
	return ln, p, nil
}

// writePortHint 把实际监听端口写到 ~/rescene_data/api_port，前端 vite dev
// 没显式给 AURORA_API_TARGET 时读它，避免端口被占后 proxy 还打 8080。
func writePortHint(port string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, "rescene_data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "api_port"), []byte(port), 0o644)
}
