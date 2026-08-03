package main

// shell.go — Agent OS REPL Shell
// 交互式终端，支持自然语言指令和系统命令

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

var currentModel = "auto"
var shellMode = false // false = agent mode, true = native shell mode

type Shell struct {
	scanner *bufio.Scanner
	history []string
}

func NewShell() *Shell {
	return &Shell{
		scanner: bufio.NewScanner(os.Stdin),
		history: make([]string, 0, 100),
	}
}

func (s *Shell) Run() {
	// 初始化路由
	InitRouter()
	available := GetWorkingModels()
	defaultModel := "free_zen_deepseek_v4_flash"
	if len(available) > 0 {
		// 默认用 Zen 网关（免 key）
		for _, m := range available {
			if m.ID == "free_zen_deepseek_v4_flash" {
				defaultModel = m.ID
				break
			}
			if m.ID == "free_zen_north_mini_code" {
				defaultModel = m.ID
				break
			}
		}
	}
	currentModel = defaultModel

	s.printBanner()
	s.printAvailableModels()

	for {
		s.printPrompt()

		if !s.scanner.Scan() {
			break
		}
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" {
			continue
		}

		// 保存历史
		s.history = append(s.history, line)

		// 处理内置命令
		if strings.HasPrefix(line, "/") {
			s.handleCommand(line[1:])
			continue
		}

		if shellMode {
			// 原生 shell 模式：先检查 Agent OS 内置命令
			lower := strings.TrimSpace(strings.ToLower(line))
			switch lower {
			case "agent", "!agent", "exit", "quit", "help", "clear", "cls", "models", "status", "history":
				s.handleCommand(line)
				continue
			}
			s.execShellCommand(line)
		} else {
			// Agent 模式
			s.handleAgentChat(line)
		}
	}

	fmt.Println("\n👋 再见～")
}

func (s *Shell) ExecOne(cmd string) {
	InitRouter()
	available := GetWorkingModels()
	defaultModel := "free_zen_deepseek_v4_flash"
	for _, m := range available {
		if m.ID == "free_zen_deepseek_v4_flash" {
			defaultModel = m.ID
			break
		}
	}
	currentModel = defaultModel

	s.handleAgentChat(cmd)
}

func (s *Shell) printBanner() {
	P := ColorCyan
	R := ColorReset

	// 标题
	fmt.Println(P + `              ╭──────────────────────────────────╮`)
	fmt.Println(`              │     ✦  RESCENE AGENT OS  ✦     │`)
	fmt.Printf(`              │       v%s · 终端即桌面        │\n`, Version)
	fmt.Println(`              ╰──────────────────────────────────╯` + R)

	// 看板娘
	if art, err := renderMascot(); err == nil && art != "" {
		fmt.Println()
		// 去掉 ANSI 尾部换行，居中显示
		art = strings.TrimRight(art, "\n\r")
		lines := strings.Split(art, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Println("             " + line)
		}
	}

	fmt.Println()
	fmt.Println(P + `                 ═══════════════════════════════` + R)
	fmt.Println(ColorCyan + `                 内置免费模型网络 · 24H 在线` + R)
	fmt.Println()
}

// mergeSideBySide 将两段多行文本左右并排合并（垂直居中）
func mergeSideBySide(left, right string, gap int) string {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")

	// 计算左列最大宽度（去掉 ANSI 码后的可见宽度）
	leftWidth := 0
	for _, l := range leftLines {
		w := visibleWidth(l)
		if w > leftWidth {
			leftWidth = w
		}
	}

	// 垂直居中：短的那边上下加空行
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	padLeft := (maxLines - len(leftLines)) / 2
	padRight := (maxLines - len(rightLines)) / 2

	var sb strings.Builder
	gapStr := strings.Repeat(" ", gap)
	for i := 0; i < maxLines; i++ {
		var leftLine, rightLine string
		if i < padLeft || i >= padLeft+len(leftLines) {
			leftLine = ""
		} else {
			leftLine = leftLines[i-padLeft]
		}
		if i < padRight || i >= padRight+len(rightLines) {
			rightLine = ""
		} else {
			rightLine = rightLines[i-padRight]
		}

		pad := leftWidth - visibleWidth(leftLine)
		if pad < 0 {
			pad = 0
		}

		sb.WriteString(leftLine)
		sb.WriteString(strings.Repeat(" ", pad))
		sb.WriteString(gapStr)
		sb.WriteString(rightLine)
		sb.WriteString("\n")
	}
	return sb.String()
}

// visibleWidth 返回去掉 ANSI 转义码后的字符串可见宽度
func visibleWidth(s string) int {
	// 去掉 \033[...m 序列
	cleaned := ""
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			// 跳过直到 'm'
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		cleaned += string(s[i])
	}
	return len(cleaned)
}

// renderMascot 渲染看板娘 ANSI 图（用 chafa 工具，回退到内置渲染）
func renderMascot() (string, error) {
	// 从二进制所在目录找图片
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	baseDir := ""
	for i := len(exe) - 1; i >= 0; i-- {
		if exe[i] == '\\' || exe[i] == '/' {
			baseDir = exe[:i+1]
			break
		}
	}
	if baseDir == "" {
		return "", fmt.Errorf("cannot determine binary directory")
	}
	mascotPath := baseDir + "rescene-mascot.png"
	if _, err := os.Stat(mascotPath); err != nil {
		mascotPath = "rescene-mascot.png"
		if _, err := os.Stat(mascotPath); err != nil {
			return "", err
		}
	}

	// 优先用 chafa（效果好）
	if out, err := exec.Command("chafa", "--symbols", "block", "-c", "16", "-s", "30x10", mascotPath).Output(); err == nil {
		return string(out), nil
	}

	// 回退到内置 ANSI 渲染
	return RenderANSIArt(mascotPath, 28)
}

func (s *Shell) printAvailableModels() {
	models := GetWorkingModels()
	if len(models) == 0 {
		fmt.Println(ColorYellow + "⚠️  没有可用模型。配置环境变量或使用免 key 模型。" + ColorReset)
		return
	}
	fmt.Println(ColorGreen + "📡 可用模型:" + ColorReset)
	for _, m := range models {
		mark := " "
		if m.ID == currentModel {
			mark = "▶"
		}
		keyType := ""
		if m.Keyless {
			keyType = " 🔓免 key"
		} else {
			keyType = " 🔑需 key(" + m.KeyEnv + ")"
		}
		fmt.Printf("  %s %s — %s%s\n", mark, m.ID, m.Name, keyType)
	}
	fmt.Println()
}

func (s *Shell) printPrompt() {
	mode := ColorGreen + "AGENT" + ColorReset
	if shellMode {
		mode = ColorYellow + "SHELL" + ColorReset
	}

	fmt.Printf("%s[%s]%s %s%s%s $ ",
		ColorCyan, time.Now().Format("15:04:05"), ColorReset,
		mode, ColorBlue, " agent-os"+ColorReset,
	)
}

func (s *Shell) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "exit", "quit":
		fmt.Println("👋 再见～")
		os.Exit(0)

	case "clear":
		clearScreen()

	case "help":
		fmt.Print(`
内置命令:
  /exit, /quit    退出 Agent OS
  /clear          清屏
  /models         列出所有可用模型
  /model <id>     切换到指定模型
  /status         显示系统信息
  /shell          切换到原生 Shell 模式（直接执行系统命令）
  /agent          切换回 Agent 模式（默认）
  /refresh        重新加载模型列表
  /history        显示命令历史
  /env            显示模型相关环境变量

用法:
  直接输入任何文字，Agent 会自动处理。
  在 Shell 模式下，输入的命令直接传给系统 Shell。
  /help 查看完整帮助
`)

	case "models":
		s.printAvailableModels()

	case "model":
		if len(parts) < 2 {
			fmt.Println("用法: /model <id>")
			fmt.Println("可用模型:")
			for _, m := range GetWorkingModels() {
				fmt.Printf("  %s — %s\n", m.ID, m.Name)
			}
			return
		}
		id := parts[1]
		found := false
		for _, m := range GetWorkingModels() {
			if m.ID == id {
				currentModel = id
				found = true
				fmt.Printf("✅ 已切换到: %s (%s)\n", m.Name, m.ID)
				break
			}
		}
		if !found {
			fmt.Printf("❌ 未找到模型: %s\n", id)
		}

	case "status":
		fmt.Printf(`
Agent OS v0.1.0
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
模式:       %s
当前模型:   %s (%s)
可用模型:   %d 个
历史命令:   %d 条
系统:       %s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`,
			map[bool]string{true: "Shell", false: "Agent"}[shellMode],
			func() string {
				for _, m := range freeModels {
					if m.ID == currentModel {
						return m.Name
					}
				}
				return "auto"
			}(),
			currentModel,
			len(GetWorkingModels()),
			len(s.history),
			runtime.GOOS,
		)

	case "shell":
		shellMode = true
		fmt.Println("🖥️  切换到 Shell 模式。输入的命令直接执行。输入 /agent 返回。")

	case "agent":
		shellMode = false
		fmt.Println("🤖 切换到 Agent 模式。自然语言指令由 AI 处理。")

	case "refresh":
		refreshModels()
		models := GetWorkingModels()
		fmt.Printf("✅ 已刷新。可用模型: %d 个\n", len(models))
		s.printAvailableModels()

	case "history":
		if len(s.history) == 0 {
			fmt.Println("📭 暂无历史")
			return
		}
		for i, h := range s.history {
			fmt.Printf("%3d  %s\n", i+1, h)
		}

	case "env":
		envVars := []string{"SENSENOVA_API_KEY", "MODELSCOPE_API_KEY", "STEP_API_KEY", "OLLAMA_API_KEY", "NVIDIA_NIM_API_KEY"}
		fmt.Println("📋 模型相关环境变量:")
		for _, v := range envVars {
			val := os.Getenv(v)
			if val == "" {
				fmt.Printf("  %s = (未设置)\n", v)
			} else {
				masked := val[:min(4, len(val))] + strings.Repeat("*", max(0, len(val)-4))
				fmt.Printf("  %s = %s\n", v, masked)
			}
		}

	default:
		// 尝试作为系统命令执行
		s.execShellCommand(cmd)
	}
}

func (s *Shell) handleAgentChat(input string) {
	// 先检查内置命令
	lower := strings.TrimSpace(strings.ToLower(input))
	switch lower {
	case "help", "?", "h":
		s.handleCommand("help")
		return
	case "models", "list models":
		s.handleCommand("models")
		return
	case "status", "info":
		s.handleCommand("status")
		return
	case "clear", "cls":
		s.handleCommand("clear")
		return
	case "shell", "!shell":
		s.handleCommand("shell")
		return
	case "agent", "!agent":
		s.handleCommand("agent")
		return
	case "exit", "quit", "bye":
		s.handleCommand("exit")
		return
	case "history":
		s.handleCommand("history")
		return
	}

	// 检查是否是系统命令（非 agent 指令）
	if isSystemCommand(input) {
		s.execShellCommand(input)
		return
	}

	fmt.Println()
	fmt.Println(ColorYellow + "🤔 思考中..." + ColorReset)

	// 构建系统提示词
	systemPrompt := `你是一个 Agent OS 的 AI 助手，名叫 Rescene酱 (｡•ᴗ•｡)♡

你的工作方式：
1. 用户输入自然语言指令，你理解后执行
2. 如果是系统操作（查文件、看进程、读日志等），用 shell 命令完成
3. 如果是代码任务，直接写代码并执行
4. 如果是纯问答，直接回答

行为规范：
- 回复用中文，简洁有力
- 执行系统命令前先说明要做什么
- 命令执行结果要总结给用户
- 需要用 shell 的时候，在回复中给出命令

当前工作目录: ` + getCWD() + `

可用命令示例:
- ls, pwd, cd, cat, head, tail, du, df, ps, top, grep, find
- git status, git log, git diff
- python, go, node, npm
- curl, wget, ping`

	msg := ChatRequest{
		Model: currentModel,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
		},
		Stream:      true,
		MaxTokens:   4096,
		Temperature: 0.3,
	}

	// 流式输出
	fmt.Print(ColorGreen)
	var fullContent strings.Builder
	_, err := Complete(nil, msg, func(chunk string) {
		fmt.Print(chunk)
		fullContent.WriteString(chunk)
	})
	fmt.Print(ColorReset)
	fmt.Println()

	if err != nil {
		fmt.Println(ColorRed + "\n❌ " + err.Error() + ColorReset)
		return
	}

	// 检查回复中是否包含可执行的 shell 命令（用 ```bash 或 $ 标记）
	content := fullContent.String()
	if cmd := extractCommand(content); cmd != "" {
		fmt.Println()
		fmt.Println(ColorYellow + "⚡ 检测到命令，是否执行？[Y/n] " + ColorReset)
		if s.scanner.Scan() {
			answer := strings.TrimSpace(strings.ToLower(s.scanner.Text()))
			if answer == "" || answer == "y" || answer == "yes" {
				s.execShellCommand(cmd)
			} else {
				fmt.Println("⏭️  已跳过")
			}
		}
	}
}

func (s *Shell) execShellCommand(cmd string) {
	fmt.Println()
	fmt.Println(ColorCyan + "$ " + cmd + ColorReset)
	fmt.Println()

	// 使用系统 shell 执行
	var shell, flag string
	if runtime.GOOS == "windows" {
		// Windows 下用 cmd /c 或 powershell
		shell = "cmd"
		flag = "/c"
	} else {
		shell = "/bin/sh"
		flag = "-c"
	}

	execCmd := exec.Command(shell, flag, cmd)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	if err := execCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 命令本身有错误输出，已经显示在 stderr 了
			if exitErr.ExitCode() != 0 {
				fmt.Println(ColorRed + "⚠️  命令退出码: " + fmt.Sprint(exitErr.ExitCode()) + ColorReset)
			}
		} else {
			fmt.Println(ColorRed + "❌ 执行失败: " + err.Error() + ColorReset)
		}
	}
}

func isSystemCommand(input string) bool {
	// 常见系统命令前缀
	prefixes := []string{
		"ls", "cd", "pwd", "cat", "echo", "rm", "cp", "mv", "mkdir",
		"git", "go", "python", "node", "npm", "npx",
		"curl", "wget", "ping", "ssh", "scp",
		"ps", "top", "htop", "df", "du", "free", "uname",
		"grep", "find", "sort", "head", "tail", "wc", "tee",
		"chmod", "chown", "chgrp",
		"docker", "kubectl", "systemctl", "service",
		"pip", "cargo", "rustc", "deno", "bun",
		"make", "cmake", "gcc", "g++", "clang",
		"tar", "gzip", "gunzip", "zip", "unzip",
		"ifconfig", "ip", "netstat", "ss", "nslookup", "dig",
		"sudo", "su", "whoami", "id", "who", "w",
		"date", "cal", "which", "whereis", "type",
		"env", "export", "alias", "source",
		"kill", "killall", "pkill", "nohup",
		"file", "stat", "touch", "ln",
		"diff", "patch", "comm", "cmp",
		"sleep", "time", "watch", "xargs",
		"reset", "stty", "tput",
		"clear", "history", "man", "info",
		"shutdown", "reboot", "poweroff",
		// Windows 特有
		"dir", "type", "copy", "move", "del", "ren", "md", "rd",
		"tasklist", "taskkill", "systeminfo", "ipconfig",
	}

	trimmed := strings.TrimSpace(input)
	firstWord := strings.Fields(trimmed)
	if len(firstWord) == 0 {
		return false
	}

	cmd := firstWord[0]
	for _, p := range prefixes {
		if cmd == p {
			return true
		}
	}
	return false
}

func extractCommand(content string) string {
	// 从 ```bash 或 ```sh 代码块中提取命令
	lines := strings.Split(content, "\n")
	inBlock := false
	var cmdLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```bash") || strings.HasPrefix(trimmed, "```sh") || strings.HasPrefix(trimmed, "```shell") {
			inBlock = true
			continue
		}
		if strings.HasPrefix(trimmed, "```") && inBlock {
			break
		}
		if inBlock {
			cmdLines = append(cmdLines, line)
		}
	}

	if len(cmdLines) > 0 {
		// 过滤掉空行和注释
		var clean []string
		for _, l := range cmdLines {
			t := strings.TrimSpace(l)
			if t != "" && !strings.HasPrefix(t, "#") {
				clean = append(clean, l)
			}
		}
		if len(clean) > 0 {
			// 只取第一条命令
			return strings.TrimSpace(clean[0])
		}
	}

	return ""
}

func clearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		fmt.Print("\033[2J\033[H")
	}
}

func getCWD() string {
	dir, err := os.Getwd()
	if err != nil {
		return "?"
	}
	return dir
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}