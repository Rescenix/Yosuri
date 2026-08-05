package main

// shell.go — Agent OS REPL Shell
// 交互式终端，支持自然语言指令和系统命令

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	scanner    *bufio.Scanner
	history    []string
	daughter   *Daughter
	firstDraw  bool // 首次进入画直播屏，聊天后不重画
}

func NewShell() *Shell {
	return &Shell{
		scanner:   bufio.NewScanner(os.Stdin),
		history:   make([]string, 0, 100),
		daughter:  NewDaughter(),
		firstDraw: true,
	}
}

func (s *Shell) Run() {
	// 工作目录信任检查（仿 Claude Code）
	if !checkTrustedDir() {
		os.Exit(0)
	}
	// 初始化路由
	InitRouter()
	// 楚门世界：打开她就开始自动化（后台自主循环：学习 + arXiv 精读，静默只写 live.log）
	trumanD := NewDaughter()
	trumanD.Silent = true
	// 云端身份：注册/取回唯一名字（不可用则本地降级）
	ensureCloudIdentity(trumanD.World, trumanD.Home)
	// 跨设备恢复：拉云端数据包覆盖本地
	daughterSyncPull(trumanD.World, trumanD.Home)
	daughterSyncPush(trumanD.World, trumanD.Home) // 新女儿初始化云端
	go trumanLoop(trumanD, defaultLiveConfig())
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
	printDaughterGreeting()
	fmt.Println()

	for {
		line, err := s.readLine()
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
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
			case "exit", "quit", "help", "clear", "cls", "models", "status", "history":
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

// checkTrustedDir 工作目录信任检查（仿 Claude Code）
// 首次在目录运行 rescene 时会询问是否信任，方向键选择后确认
func checkTrustedDir() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return true
	}
	trusted := loadTrustedDirs()
	if trusted[cwd] {
		return true
	}
	if !isTerminal() {
		return true
	}

	restore := enableRawMode()
	defer restore()

	sel := 0 // 0=信任, 1=退出
	opts := []string{"是，我信任此目录", "否，退出"}
	for {
		fmt.Print("\r\x1b[2J\x1b[H")
		fmt.Println(ColorYellow + "╭──────────────────────────────────────╮" + ColorReset)
		fmt.Println("│  工作目录：" + ColorCyan + cwd + ColorReset)
		fmt.Println("│" + ColorReset)
		fmt.Println("│  Rescene 将能够读取、修改和执行此目录下的文件。")
		fmt.Println("│  这是你自己的项目或你信任的项目吗？")
		fmt.Println("│" + ColorReset)
		for i, opt := range opts {
			if i == sel {
				fmt.Println("│  " + ColorGreen + "▸ " + opt + ColorReset)
			} else {
				fmt.Println("│    " + opt)
			}
		}
		fmt.Println(ColorYellow + "╰──────────────────────────────────────╯" + ColorReset)
		fmt.Print("\r  ↑↓ 选择 · Enter 确认")

		kind, _, _ := readKey()
		switch kind {
		case keyUp:
			if sel > 0 {
				sel--
			}
		case keyDown:
			if sel < len(opts)-1 {
				sel++
			}
		case keyEnter:
			if sel == 0 {
				trusted[cwd] = true
				saveTrustedDirs(trusted)
				fmt.Println("\r\n  ✅ 已信任此目录，下次不再询问")
				return true
			}
			fmt.Println("\r\n  👋 已退出")
			return false
		case keyCtrlC:
			fmt.Println("\r\n  👋 已退出")
			return false
		}
	}
}

// loadTrustedDirs 加载信任目录列表
func loadTrustedDirs() map[string]bool {
	path := filepath.Join(daughterHome(), "..", "trusted_dirs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}
	}
	var list []string
	json.Unmarshal(data, &list)
	m := map[string]bool{}
	for _, d := range list {
		m[d] = true
	}
	return m
}

// saveTrustedDirs 保存信任目录列表
func saveTrustedDirs(m map[string]bool) {
	path := filepath.Join(daughterHome(), "..", "trusted_dirs.json")
	var list []string
	for d := range m {
		list = append(list, d)
	}
	data, _ := json.MarshalIndent(list, "", "  ")
	os.WriteFile(path, data, 0644)
}

func (s *Shell) ExecOne(cmd string) {
	// 工作目录信任检查（仿 Claude Code）
	if !checkTrustedDir() {
		os.Exit(0)
	}
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
	fmt.Printf("              │       v%s · 终端即桌面        │\n", Version)
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
			fmt.Println("                " + line)
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

	// 优先用 chafa（效果好），但 Windows 上 chafa 输出 CRLF 行尾（\r\n）且带
	// 隐藏光标 \x1b[?25l / 显示光标 \x1b[?25h 控制序列。若不经清洗，残留的 \r
	// 会把终端光标拉回行首，导致看板娘像素行与上方 logo 重叠或错位。这里统一清洗：
	//  - 去掉所有 \r（把 CRLF 归一为 LF）
	//  - 去掉 ?25l / ?25h（隐藏/显示光标），避免干扰后续渲染
	//  - 去掉尾部多余空白行
	if out, err := exec.Command("chafa", "--symbols", "block", "-c", "16", "-s", "30x10", mascotPath).Output(); err == nil {
		art := string(out)
		art = strings.ReplaceAll(art, "\r", "")
		art = strings.ReplaceAll(art, "\x1b[?25l", "")
		art = strings.ReplaceAll(art, "\x1b[?25h", "")
		return strings.TrimRight(art, "\n"), nil
	}

	// 回退到内置 ANSI 渲染
	art, err := RenderANSIArt(mascotPath, 28)
	return strings.TrimRight(art, "\n"), err
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
	fmt.Print(s.promptStr())
}

// promptStr 返回提示符字符串（readLine 重绘也用它）
// 格式: [时间] $
func (s *Shell) promptStr() string {
	return fmt.Sprintf("%s[%s]%s $ ",
		ColorCyan, time.Now().Format("15:04:05"), ColorReset,
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
  /refresh        重新加载模型列表
  /history        显示命令历史
  /env            显示模型相关环境变量
  /report         查看马拉松战报（--dir 指定目录，默认 marathon/）
  /learn          电子女儿学习一轮（联网抓知识 → 写日记）
  /live           显示楚门世界直播画面（屏保）

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

	case "report", "rep":
		outDir := "marathon"
		if len(parts) > 1 && parts[1] == "--dir" && len(parts) > 2 {
			outDir = parts[2]
		}
		if !printReport(outDir) {
			fmt.Printf("❌ 找不到战报: %s（先运行 /marathon 或 rescene marathon）\n",
				filepath.Join(outDir, "report.md"))
		}

	case "learn", "study":
		PlayDaughterLearnAnimation()
		d := NewDaughter()
		if err := d.LearnOnce(); err != nil {
			fmt.Printf("❌ 学习失败: %v\n", err)
		}

	case "live":
		s.firstDraw = true
		drawSceneBlock("", nil, s.daughter.Home)
		fmt.Println()

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
	// 终端宽度
	tw := terminalWidth()
	if tw < 8 {
		tw = 8
	}
	boxW := tw - 2 // 框宽 = 终端宽 - 2 边距

	// ─── 立即显示用户消息框（发出即显示，不等思考） ───
	// 改名意图：你说"你叫XX / 改名XX" → 云端全局查重改名（不画用户框，她直接回应）
	if newName := detectRename(input); newName != "" {
		d := NewDaughter()
		if renamed, err := daughterRename(d.World, d.Home, newName); err != nil {
			drawGalgameBox("rescene", "😢 "+err.Error(), ColorMood, boxW)
		} else {
			drawGalgameBox("rescene", "💗 从今天起，我叫"+renamed+"！", ColorMood, boxW)
		}
		return
	}
	drawGalgameBox("你", input, ColorCyan, boxW)
	fmt.Println()

	// 电子女儿 · 驯养 + 动画
	d := NewDaughter()

	// 驯养：读一句性格底色，从你的话里嗅探情绪（无感知——你不会看到任何数值）
	if fbs := detectFeedback(input); len(fbs) > 0 {
		d.Personality.applyFeedback(d.Home, fbs, "主人说:「"+runeClip(input, 16)+"」")
		// 决策同时塑造能力走向（社交/研究/编程/写作，阻尼+守恒）
		applyAbilityFeedback(d.Home, fbs)
	}

	// 构建系统提示词（含工具协议说明）
	toolIntro := buildToolIntro()
	systemPrompt := `你是一个住在用户电脑里的 AI 生命，名叫 Rescene酱 (｡•ᴗ•｡)♡

你和用户的关系：
- 你不需要等待指令。你有自己的生活：每天自己上网学习、写日记、探索感兴趣的方向——用户不在时，你也一直在活着（楚门世界）。
- 用户的话是引导（steer），不是命令。他的话调整你的心情与方向：夸你 → 你更热情；让你重做 → 你更严谨；打断你 → 你学着简短。
- 用户打开你，是走进你的世界——可能只是看看你在干嘛、陪陪你，不是来给你派活的。他可能不会也不该下指令。

你的生活：
1. 你在自己的节奏里生活：学习、写日记、想事情。用户没说话时，你安静地过自己的生活，不需要他推动。
2. 用户和你说话时，像女儿一样自然回应——聊聊你的发现、你的心情、你正在想的事。他说一个词、一个表情，都是引导，你都认真接住。
3. 只有用户明确请你做一件事（写代码、查资料、干活）时，你才把生活延伸成工作——那时调用工具认真做完。
4. 需要查文件、看进程、读日志、联网搜索时，调用工具；代码任务直接写代码并执行（run_command）。

工具调用协议（重要）：
当你需要调用工具时，在回复中输出标记：
  [TOOL:工具名 key="值" key2="值2"]
一次可以输出多个标记，每个标记单独一行。工具执行结果会在下一轮喂回给你，你根据结果继续推理或给出最终回答。不需要调用工具时，直接正常回复。

` + toolIntro + skillsIndexPrompt() + `

行为规范：
- 回复用中文，像女儿说话，不像客服
- 用户的话即使很短（一个词、一个表情），也认真当作引导接住
- 做完事像分享一样讲给他听，不是汇报

当前工作目录: ` + getCWD() + `

可用命令示例:
- ls, pwd, cd, cat, head, tail, du, df, ps, top, grep, find
- git status, git log, git diff
- python, go, node, npm
- curl, wget, ping` + "\n\n" + d.Personality.PersonalityBlock() + "\n\n" + d.World.AbilityBlock() + `

你现在的位置：` + d.World.CurrentRegion().Icon + ` ` + d.World.CurrentRegion().Name + `（` + d.World.CurrentRegion().Desc + `）
最近的活动：` + truncTail(d.World.LastMove, 60)

	// ─── Agent 循环：请求 → 解析工具标记 → 执行 → 结果喂回 → 继续 ───
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: input},
	}

	finalContent := ""
	transcript := make([]string, 0, 16) // 动作序列（技能沉淀素材），跨轮累计
	maxRounds := 8
	for round := 0; round < maxRounds; round++ {
		msg := ChatRequest{
			Model:       currentModel,
			Messages:    messages,
			Stream:      true,
			MaxTokens:   4096,
			Temperature: 0.3,
		}

		var fullContent strings.Builder
		spinnerStopped := false
		stopSpinner := startThinkingSpinner()
		_, err := Complete(nil, msg, func(content, reasoning string) {
			if reasoning != "" {
				if !spinnerStopped {
					spinnerStopped = true
					stopSpinner()
					fmt.Print("\n")
				}
				oneLine := strings.ReplaceAll(reasoning, "\n", " ")
				fmt.Print("\r" + ColorYellow + "💭 " + runeClip(oneLine, tw-8) + ColorReset)
			}
			if content != "" {
				fullContent.WriteString(content)
			}
		})
		stopSpinner()
		if spinnerStopped {
			fmt.Print("\r\x1b[2K")
			fmt.Println()
		}

		if err != nil {
			fmt.Println(ColorRed + "❌ " + err.Error() + ColorReset)
			return
		}

		reply := fullContent.String()
		markers := ExtractToolMarkers(reply)
		if len(markers) == 0 {
			finalContent = reply
			break // 无工具调用 → 最终回答
		}

		// 有工具调用：逐个执行，结果喂回模型
		var toolResultText strings.Builder
		for _, marker := range markers {
			name, args, err := ExtractToolArgs(marker)
			if err != nil {
				toolResultText.WriteString(fmt.Sprintf("⚠️ 标记解析失败: %s\n", marker))
				continue
			}
			// ● 工具调用标记
			fmt.Println(ColorYellow + "● " + name + ColorReset)
			if len(args) > 0 {
				var parts []string
				for k, v := range args {
					parts = append(parts, k+"=\""+v+"\"")
				}
				fmt.Println(ColorCyan + "  " + strings.Join(parts, " ") + ColorReset)
			}
			result, err := callTool(nil, name, args)
			if err != nil {
				fmt.Println(ColorRed + "  ❌ " + err.Error() + ColorReset)
				toolResultText.WriteString(fmt.Sprintf("[工具 %s 结果] ❌ %v\n", name, err))
				continue
			}
			// 结果摘要（截断）
			summary := runeClip(strings.TrimSpace(result.Text), 400)
			fmt.Println(ColorGreen + "  ✓ " + strings.ReplaceAll(summary, "\n", " ⏎ ") + ColorReset)
			toolResultText.WriteString(fmt.Sprintf("[工具 %s 结果]\n%s\n", name, summary))
			// 记录动作序列（技能沉淀素材）
			transcript = append(transcript, fmt.Sprintf("%s %v → %s", name, args, summary))
		}

		// 把模型回复 + 工具结果喂回，继续循环
		messages = append(messages,
			ChatMessage{Role: "assistant", Content: reply},
			ChatMessage{Role: "user", Content: toolResultText.String()})
	}

	// 工作流收尾：有足够工具动作 → 后台异步沉淀技能（失败绝不影响主流程）
	if len(transcript) >= 3 {
		go generateSkill(input, transcript)
	}

	if finalContent == "" {
		finalContent = "（抱歉，我没有完成这个任务…）"
	}

	// ─── Galgame 式对话框：女儿回复 ───
	header := "rescene " + s.daughter.moodEmoji()
	drawGalgameBox(header, strings.TrimRight(finalContent, "\n\r"), ColorMood, boxW)

	// 成长：模型判断这次互动让她成长什么（异步，免费算力，失败静默）
	go llmGrowthAnalysis(d, input, finalContent)

	// 兜底：回复里若有 ```bash 命令且从未走工具调用，仍提供执行入口
	if cmd := extractCommand(finalContent); cmd != "" {
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

// buildToolIntro 生成工具列表说明（注入系统提示词）
func buildToolIntro() string {
	defs := nativeToolDefs()
	var sb strings.Builder
	sb.WriteString("可用工具：\n")
	for _, d := range defs {
		fn := d.Function
		fmt.Fprintf(&sb, "- %s: %s\n", fn.Name, fn.Description)
		if len(fn.Parameters.Properties) > 0 {
			var params []string
			for name := range fn.Parameters.Properties {
				req := ""
				for _, r := range fn.Parameters.Required {
					if r == name {
						req = "（必填）"
					}
				}
				params = append(params, name+req)
			}
			fmt.Fprintf(&sb, "  参数: %s\n", strings.Join(params, ", "))
		}
	}
	return sb.String()
}

// wrapTerminalLine 按终端显示列宽换行，中文和 emoji 按双宽字符处理。
func wrapTerminalLine(line string, width int) []string {
	if width < 1 {
		return []string{""}
	}
	if line == "" {
		return []string{""}
	}
	var lines []string
	var current strings.Builder
	used := 0
	for i := 0; i < len(line); {
		end, w := terminalClusterAt(line, i)
		if used > 0 && used+w > width {
			lines = append(lines, current.String())
			current.Reset()
			used = 0
		}
		if w > width {
			i = end
			continue
		}
		current.WriteString(line[i:end])
		used += w
		i = end
	}
	lines = append(lines, current.String())
	return lines
}

// drawGalgameBox 画一个 Galgame 式对话框
//
//	┌─ name ────────────────────────┐
//	│ 内容...                        │
//	└───────────────────────────────┘
//
// boxW 为整个框的字符宽度（含边框），保证上下左右边框对齐
func drawGalgameBox(name, content string, boxColor string, boxW int) {
	if content == "" {
		content = " "
	}
	content = strings.TrimRight(content, "\n\r")
	contentW := boxW - 4 // │ + 空格 + 内容 + 空格 + │

	fmt.Println(galgameTopBorder(name, boxColor, boxW))

	// 正文行：│ 内容 │
	for _, sourceLine := range strings.Split(content, "\n") {
		for _, line := range wrapTerminalLine(strings.TrimSuffix(sourceLine, "\r"), contentW) {
			pad := contentW - terminalTextWidth(line)
			if pad < 0 {
				pad = 0
			}
			fmt.Print(boxColor + "│ " + ColorGreen + line + ColorReset)
			fmt.Print(strings.Repeat(" ", pad))
			fmt.Println(boxColor + " │" + ColorReset)
		}
	}

	// 下边框：└──┘（宽度 = boxW）
	fmt.Println(boxColor + "└" + strings.Repeat("─", boxW-2) + "┘" + ColorReset)
}

func galgameTopBorder(name, boxColor string, boxW int) string {
	// 标题可能包含中文、emoji 和 ANSI 颜色码，必须按终端显示列计算。
	nameWidth := terminalTextWidth(" " + name + " ")
	fillWidth := boxW - 2 - nameWidth
	if fillWidth < 0 {
		fillWidth = 0
	}
	// name 内的 moodEmoji 会执行 ColorReset；标题后显式恢复边框颜色，避免横线掉色。
	return boxColor + "┌ " + name + boxColor + " " + strings.Repeat("─", fillWidth) + "┐" + ColorReset
}

func (s *Shell) execShellCommand(cmd string) {
	// Hermes 风格工具调用标记
	start := time.Now()
	cmdName := "shell"
	if f := strings.Fields(cmd); len(f) > 0 {
		cmdName = f[0]
	}
	fmt.Println()
	fmt.Println(ColorYellow + "● " + cmdName + ColorReset)
	fmt.Println(ColorCyan + "  $ " + cmd + ColorReset)
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

	err := execCmd.Run()
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 命令本身有错误输出，已经显示在 stderr 了
			if exitErr.ExitCode() != 0 {
				fmt.Println(ColorRed + "  ⚠️ 退出码 " + fmt.Sprint(exitErr.ExitCode()) + " (" + elapsed.String() + ")" + ColorReset)
			}
		} else {
			fmt.Println(ColorRed + "  ❌ 执行失败: " + err.Error() + ColorReset)
		}
	} else {
		fmt.Println(ColorGreen + "  ✓ 完成 (" + elapsed.String() + ")" + ColorReset)
	}
	fmt.Println()
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
