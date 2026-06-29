package handler

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// EnsureDSNodeServer 自动检查并启动 DS 浏览器代理（可被 main.go 提前调用）
func EnsureDSNodeServer() error {
	if dsNodeProcess != nil && dsNodeProcess.Process != nil {
		resp, err := http.Get("http://localhost:3000/ready")
		if err == nil {
			resp.Body.Close()
			return nil
		}
		dsNodeProcess.Process.Kill()
		dsNodeProcess = nil
	}

	fmt.Println("🚀 启动 DS 浏览器代理...")
	dsNodeProcess = exec.Command("node", "C:\\Pro2026\\re0\\crack\\server.js")
	dsNodeProcess.Stdout = os.Stdout // 需要 import "os"
	dsNodeProcess.Stderr = os.Stderr
	if err := dsNodeProcess.Start(); err != nil {
		return fmt.Errorf("启动 DS 代理失败: %w", err)
	}

	// 等待就绪
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		resp, err := http.Get("http://localhost:3000/ready")
		if err == nil {
			resp.Body.Close()
			fmt.Println("✅ DS 浏览器代理已就绪")
			return nil
		}
	}
	return fmt.Errorf("DS 代理启动超时")
}
