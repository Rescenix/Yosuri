package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 子命令模式
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "exec", "e", "run":
			// agent-os exec "帮我查下磁盘空间"
			cmd := ""
			if len(os.Args) > 2 {
				cmd = os.Args[2]
			}
			oneShot(cmd)
			return
		case "help", "--help", "-h":
			printHelp()
			return
		case "version", "--version", "-v":
			fmt.Println("Agent OS v0.1.0 — Rescene 内核")
			return
		}
	}

	// 解析 flags
	flag.Usage = func() {
		printHelp()
	}
	flag.Parse()

	// 启动信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\n👋 Agent OS 已关闭。下次见～")
		os.Exit(0)
	}()

	// 启动 REPL
	shell := NewShell()
	shell.Run()
}

func oneShot(cmd string) {
	if cmd == "" {
		fmt.Println("用法: agent-os exec \"你的指令\"")
		os.Exit(1)
	}
	shell := NewShell()
	shell.ExecOne(cmd)
}

func printHelp() {
	fmt.Print(`╔══════════════════════════════════════════╗
║      Agent OS — 终端即桌面              ║
║      内置 Rescene 免费模型网络           ║
╚══════════════════════════════════════════╝

用法:
  agent-os             启动交互式 Shell
  agent-os exec "..."  单条指令执行
  agent-os help        显示帮助
  agent-os version     显示版本

内置命令:
  help        显示帮助
  exit/quit   退出
  clear       清屏
  models      列出可用模型
  model <id>  切换当前模型
  status      显示系统状态
  shell       进入原生 Shell 模式
  agent       返回 Agent 模式（默认）

在 Agent 模式下，直接输入自然语言就会由 AI 处理。
在 Shell 模式下，输入的命令直接传递给系统 Shell。

示例:
  $ 帮我查下当前目录最大的文件
  $ 写一个 Python 脚本监控 CPU 温度
  $ 帮我总结一下最近的 git 提交
`)
}