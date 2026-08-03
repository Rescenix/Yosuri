package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

const Version = "0.1.0"

func main() {
	// 子命令模式
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "exec", "e", "run":
			cmd := ""
			if len(os.Args) > 2 {
				cmd = os.Args[2]
			}
			oneShot(cmd)
			return
		case "update", "upgrade", "self-update":
			doUpdate()
			return
		case "help", "--help", "-h":
			printHelp()
			return
		case "version", "--version", "-v":
			fmt.Printf("Rescene Agent OS v%s — %s/%s\n", Version, runtime.GOOS, runtime.GOARCH)
			return
		}
	}

	flag.Usage = func() {
		printHelp()
	}
	flag.Parse()

	// 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\n👋 再见～")
		os.Exit(0)
	}()

	// 启动 REPL
	shell := NewShell()
	shell.Run()
}

func oneShot(cmd string) {
	if cmd == "" {
		fmt.Println("用法: rescene exec \"你的指令\"")
		os.Exit(1)
	}
	shell := NewShell()
	shell.ExecOne(cmd)
}

func doUpdate() {
	fmt.Println("🔄 检查更新...")
	// 先通过 install.ps1 或 install.sh 重装
	// 检测当前系统和架构
	baseURL := "https://raw.githubusercontent.com/Rescenix/ResceneAgent/main/agent-os"

	switch runtime.GOOS {
	case "windows":
		// Windows: 下载 install.ps1 并执行
		fmt.Println("📥 下载最新版本...")
		fmt.Println("  运行: powershell -c \"irm " + baseURL + "/install.ps1 | iex\"")
		fmt.Println()
		fmt.Println("或者手动下载:")
		fmt.Printf("  %s/rescene.exe\n", baseURL)
	case "linux", "darwin":
		fmt.Println("📥 下载最新版本...")
		fmt.Printf("  运行: curl -fsSL %s/install.sh | sh\n", baseURL)
		fmt.Println()
		fmt.Println("或者手动下载:")
		arch := runtime.GOARCH
		if arch == "arm64" {
			fmt.Printf("  %s/rescene-arm64 → ~/.rescene/rescene\n", baseURL)
		} else {
			fmt.Printf("  %s/rescene-linux → ~/.rescene/rescene\n", baseURL)
		}
	}
	fmt.Println()
	fmt.Println("✅ 更新完成！重启 rescene 使用新版本。")
}

func printHelp() {
	fmt.Printf(`╔══════════════════════════════════════════╗
║      Rescene Agent OS v%s           ║
║      内置免费模型网络 · 终端即桌面      ║
╚══════════════════════════════════════════╝

用法:
  rescene              启动交互式 Shell
  rescene exec "..."   单条指令执行
  rescene update       检查并更新到最新版
  rescene version      显示版本
  rescene help         显示帮助

一行安装:
  PowerShell: irm https://git.io/rescene | iex
  bash:        curl -fsSL https://git.io/rescene.sh | sh

免 key 模型开箱即用，更多模型配置环境变量:
  SENSENOVA_API_KEY    商汤免费
  MODELSCOPE_API_KEY   魔搭免费
  STEP_API_KEY         阶跃星辰免费
`, Version)
}