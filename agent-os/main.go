package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
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
		case "marathon", "m", "run24":
			runMarathon(os.Args[2:])
			return
		case "daemon", "d", "auto":
			runDaemon()
			return
		case "report", "rep":
			showReport(os.Args[2:])
			return
		case "learn", "study":
			runDaughterLearn()
			return
		case "update", "upgrade", "self-update":
			doUpdate()
			return
		case "reply":
			runReply(os.Args[2:])
			return
		case "publish", "pub":
			runPublish(os.Args[2:])
			return
		case "edge-debug", "edge":
			edgeDebugHint()
			return
		case "chrome-login", "login":
			runChromeLogin()
			return
		case "company", "org":
			runCompany(os.Args[2:])
			return
		case "company-repo", "repo":
			runCompanyRepo(os.Args[2:])
			return
		case "demo-delivery":
			runDemoDelivery()
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
		gracefulExit()
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

// runDaemon 24H 无头自转：不启动 REPL，纯后台跑 trumanLoop（Silent 只写 live.log）。
// Windows 计划任务/开机自启/后台挂起都走这里——关掉终端她也在转。
func runDaemon() {
	// 崩溃日志：daemon 意外死亡时留证据（24H 守护排障）
	defer func() {
		if r := recover(); r != nil {
			crash := fmt.Sprintf("[%s] daemon PANIC: %v\n%s\n", time.Now().Format("2006-01-02 15:04:05"), r, debug.Stack())
			if home, err := os.UserHomeDir(); err == nil {
				os.WriteFile(filepath.Join(home, "rescene_data", "daughter", "crash.log"), []byte(crash), 0o644)
			}
			fmt.Printf("❌ daemon panic: %v（详情见 crash.log）\n", r)
			os.Exit(1)
		}
	}()
	InitRouter()
	trumanD := NewDaughter()
	trumanD.Silent = true
	ensureCloudIdentity(trumanD.World, trumanD.Home)
	daughterSyncPull(trumanD.World, trumanD.Home)
	daughterSyncPush(trumanD.World, trumanD.Home)
	models := GetWorkingModels()
	fmt.Printf("🚀 Rescene 24H 自转守护已启动：第 %d 天 · %d 个免费模型可用 · 每 2 分钟一轮\n", trumanD.loadStats().Days, len(models))
	fmt.Printf("   她正在后台工作（学习/读书/技能/项目/社交/思考/日记）——Ctrl+C 停止\n")
	safeGo("daemon-probe", probeModels) // 启动即探活校准池子（异步）
	trumanLoop(trumanD, defaultLiveConfig())
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
  rescene              启动交互式 Shell（电子女儿会问候你）
  rescene daemon       24H 无头自转守护（后台自动工作，关终端也在转）
  rescene exec "..."   单条指令执行
  rescene marathon     24H 自迭代马拉松（热点立项 → 需求→计划→自检闭环）
  rescene report       查看马拉松战报（--dir 指定目录）
  rescene learn        电子女儿学习一轮（联网抓知识 → 写日记）
  rescene reply        替你回复平台评论/私信（CSDN 等）
  rescene publish      一键发布文章到网文平台（晋江/番茄/纵横/17K/七猫/飞卢/咪咕/黑岩/掌阅/豆瓣）
  rescene edge-debug   让 Edge 常驻调试端口（cookie 自动读取，浏览器不关）
  rescene company       多 Agent 公司（百人并行自转协作）
  rescene demo-delivery 生成一套通过硬门槛的全链路演示项目
    rescene chrome-login  打开发布专用 Chrome
  rescene version      显示版本
  rescene help         显示帮助

marathon 参数:
  --task "..."     用户自编排项目（跳过热点选题）
  --hours N        运行时长（默认 24）
  --interval S     每轮间隔秒（默认 60）
  --rounds N       固定轮数（优先于 --hours，测试用）
  --model <id>     固定模型（默认自动轮换）
  --hot hn|github  热点源（默认 hn）
  --iters N        每项目迭代轮数（默认 6）
  --quick          快速自测模式

一行安装:
  PowerShell: irm https://git.io/rescene | iex
  bash:        curl -fsSL https://git.io/rescene.sh | sh

免 key 模型开箱即用，更多模型配置环境变量:
  SENSENOVA_API_KEY    商汤免费
  MODELSCOPE_API_KEY   魔搭免费
  STEP_API_KEY         阶跃星辰免费
`, Version)
}
