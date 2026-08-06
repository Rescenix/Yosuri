package main

// scene.go — 24H 自转工作面板（固定高度全宽单栏）
//
// 她不是 emoji 场景秀：左栏是她的工具调用流（● 工具名 + 参数 + 结果），
// 右栏滚动日志。顶部显示她此刻的思考。整个面板保留终端最右一列，
// 避免 Windows Console/ConPTY 写满一行后自动换行。

import (
	"fmt"
	"strings"
	"time"
)

// liveSceneRows 包含上下边框；输入行始终位于这 12 行之后。
const liveSceneRows = 12

// toolEvent 她的一次工具调用（工具调用可视化）
type toolEvent struct {
	Name   string `json:"name"`             // 工具名（explore/study/skill/...）
	Args   string `json:"args,omitempty"`   // 参数摘要（dir="东" 之类）
	Status string `json:"status"`           // running | done | fail
	Result string `json:"result,omitempty"` // 结果摘要
	At     string `json:"at,omitempty"`     // HH:MM:SS
}

type sceneFrame struct {
	RegionName string   `json:"region"`
	RegionIcon string   `json:"icon"`
	RegionKind string   `json:"kind"`
	X          int      `json:"x,omitempty"`
	Y          int      `json:"y,omitempty"`
	Action     string   `json:"action"`
	Mood       string   `json:"mood"`
	TravelIcon string   `json:"travel"`
	Ability    string   `json:"ability"`
	Friend     string   `json:"friend"`
	Seed       int64    `json:"seed"`
	Thinking   string   `json:"thinking,omitempty"` // 她此刻的思考（思考可视化）
	Tools      []toolEvent `json:"-"`               // 最近工具调用流
	Version    int64    `json:"version"`
	LogLines   []string `json:"-"`
}

var (
	liveFrameMu  = syncMu()
	currentFrame = sceneFrame{RegionName: "出生地", RegionIcon: "🏠", RegionKind: "城市街区", Action: "刚刚醒来", Mood: "(◕‿◕)"}
)

func syncMu() *mu { return newMu() }

type mu struct{ ch chan struct{} }

func newMu() *mu      { return &mu{ch: make(chan struct{}, 1)} }
func (m *mu) Lock()   { m.ch <- struct{}{} }
func (m *mu) Unlock() { <-m.ch }

func updateLiveFrame(f sceneFrame) {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	f.Version = time.Now().UnixNano()
	currentFrame = f
}

// pushToolCall 记录一次工具调用（工具调用可视化：● 名 参数 + 结果）
// 慢操作（模型调用）前 push running，完成后 push done/fail。
func pushToolCall(name, args, status, result string) {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	ev := toolEvent{
		Name:   name,
		Args:   runeClip(args, 40),
		Status: status,
		Result: runeClip(result, 46),
		At:     time.Now().Format("15:04:05"),
	}
	currentFrame.Tools = append(currentFrame.Tools, ev)
	if len(currentFrame.Tools) > 10 {
		currentFrame.Tools = currentFrame.Tools[len(currentFrame.Tools)-10:]
	}
	currentFrame.Version = time.Now().UnixNano()
}

// setThinking 显示/清除她此刻的思考（思考可视化：顶部 💭 状态）
func setThinking(s string) {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	currentFrame.Thinking = s
	currentFrame.Version = time.Now().UnixNano()
}

// toolEventByName 把同一次调用的 running 记录升级为 done/fail（保留参数行）
func toolEventByName(name, status, result string) {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	for i := len(currentFrame.Tools) - 1; i >= 0; i-- {
		if currentFrame.Tools[i].Name == name && currentFrame.Tools[i].Status == "running" {
			currentFrame.Tools[i].Status = status
			currentFrame.Tools[i].Result = runeClip(result, 46)
			break
		}
	}
	currentFrame.Version = time.Now().UnixNano()
}

func currentLiveFrame(logPath string, n int) sceneFrame {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	f := currentFrame
	if logPath != "" {
		f.LogLines = liveLogTailLines(logPath, n)
	}
	return f
}

func liveFrameVersion() int64 {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	return currentFrame.Version
}

// sceneColWidthsFor 返回共享外框内左右内容区的宽度。
// 整行结构为：│ left │ right │，三个边框列之外按 3:1 分配。
func sceneColWidthsFor(terminalW int) (left, right int) {
	if terminalW < 8 {
		terminalW = 8
	}
	canvasW := terminalW - 1 // 永远不写最右一列
	contentW := canvasW - 3
	left = contentW * 3 / 4
	right = contentW - left
	if left < 1 {
		left = 1
	}
	if right < 1 {
		right = 1
		left = contentW - right
	}
	return left, right
}

func sceneColWidths() (left, right int) {
	return sceneColWidthsFor(terminalWidth())
}

// truncateTerminalText 保留 ANSI 控制序列，按实际显示列裁剪。
func truncateTerminalText(s string, width int, ellipsis bool) string {
	if width <= 0 {
		return ""
	}
	if terminalTextWidth(s) <= width {
		return s
	}
	limit := width
	suffix := ""
	if ellipsis {
		limit--
		if limit < 0 {
			return ""
		}
		suffix = "…"
	}
	var b strings.Builder
	used := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			start := i
			i += 2
			for i < len(s) {
				c := s[i]
				i++
				if c >= 0x40 && c <= 0x7e {
					break
				}
			}
			b.WriteString(s[start:i])
			continue
		}
		end, cellW := terminalClusterAt(s, i)
		if used+cellW > limit {
			break
		}
		b.WriteString(s[i:end])
		used += cellW
		i = end
	}
	// 裁剪可能发生在着色片段中，先复位，防止颜色污染边框和下一栏。
	return b.String() + ColorReset + suffix
}

func fitTerminalText(s string, width int, ellipsis bool) string {
	s = truncateTerminalText(s, width, ellipsis)
	if pad := width - terminalTextWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func sceneTopSegment(title string, width int) string {
	label := truncateTerminalText("─ "+title+" ", width, true)
	fill := width - terminalTextWidth(label)
	return label + strings.Repeat("─", maxI(0, fill))
}

func sceneSharedTop(leftTitle, rightTitle string, left, right int) string {
	return ColorCyan + "╭" + sceneTopSegment(leftTitle, left) + "┬" +
		sceneTopSegment(rightTitle, right) + "╮" + ColorReset
}

func sceneSharedBody(leftText, rightText string, left, right int) string {
	return ColorCyan + "│" + ColorReset + fitTerminalText(leftText, left, true) +
		ColorCyan + "│" + ColorReset + fitTerminalText(rightText, right, true) +
		ColorCyan + "│" + ColorReset
}

func sceneSharedBottom(left, right int) string {
	return ColorCyan + "╰" + strings.Repeat("─", left) + "┴" +
		strings.Repeat("─", right) + "╯" + ColorReset
}

func renderLiveLines(f sceneFrame) []string {
	return renderLiveLinesAtWidth(f, terminalWidth())
}

func renderLiveLinesAtWidth(f sceneFrame, terminalW int) []string {
	width := terminalW - 2 // 全宽单栏：左右边框各 1 列
	if width < 1 {
		width = 1
	}
	bodyRows := liveSceneRows - 2
	content := leftSceneLines(f)

	title := "Rescene · 24H 自转"
	if f.Thinking != "" {
		title += " · " + f.Thinking
	} else {
		title += " · " + time.Now().Format("15:04:05")
	}
	lines := make([]string, 0, liveSceneRows)
	lines = append(lines, sceneTop(title, width))
	for i := 0; i < bodyRows; i++ {
		lc := ""
		if i < len(content) {
			lc = content[i]
		}
		lines = append(lines, sceneBodyLine(lc, width))
	}
	lines = append(lines, sceneBottom(width))
	return lines
}

// sceneTop / sceneBodyLine / sceneBottom 全宽单栏边框（Hermes 工作流面板）
func sceneTop(title string, width int) string {
	label := truncateTerminalText("─ "+title+" ", width, true)
	fill := width - terminalTextWidth(label)
	return ColorCyan + "╭" + label + strings.Repeat("─", maxI(0, fill)) + "╮" + ColorReset
}

func sceneBodyLine(text string, width int) string {
	return ColorCyan + "│" + ColorReset + fitTerminalText(text, width, true) + ColorCyan + "│" + ColorReset
}

func sceneBottom(width int) string {
	return ColorCyan + "╰" + strings.Repeat("─", width) + "╯" + ColorReset
}

// leftSceneLines 全宽内容：她的 Hermes 风格工作流（最近 5 条事件）
// 💭 思考（黄）→ ● 工具名 参数（黄●+青参）→ ✓/❌ 结果（绿/红）
func leftSceneLines(f sceneFrame) []string {
	bodyRows := liveSceneRows - 2
	lines := make([]string, 0, bodyRows)

	tools := f.Tools
	if len(tools) > 5 {
		tools = tools[len(tools)-5:]
	}
	for _, t := range tools {
		argsPart := ""
		if t.Args != "" {
			argsPart = "  " + ColorCyan + t.Args + ColorReset
		}
		switch t.Status {
		case "think":
			// 💭 思考行（思考可视化）+ Args（action/model 路由）
			lines = append(lines, ColorYellow+"💭 "+t.Result+ColorReset+argsPart)
		case "running":
			lines = append(lines, ColorYellow+"● "+t.Name+ColorReset+argsPart)
			lines = append(lines, ColorYellow+"  ⏳ 进行中…"+ColorReset)
		case "fail":
			lines = append(lines, ColorYellow+"● "+t.Name+ColorReset+argsPart)
			lines = append(lines, ColorRed+"  ❌ "+t.Result+ColorReset)
		default:
			lines = append(lines, ColorYellow+"● "+t.Name+ColorReset+argsPart)
			lines = append(lines, ColorGreen+"  ✓ "+t.Result+ColorReset)
		}
	}

	// 没有事件时给一句她的状态
	if len(lines) == 0 && f.Action != "" {
		lines = append(lines, "  "+f.Action)
		lines = append(lines, "  ")
	}
	for len(lines) < bodyRows {
		lines = append(lines, "  ")
	}
	return lines[:bodyRows]
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// drawSceneBlock 首次绘制 12 行工作面板，随后在第 13 行绘制输入提示符。
func drawSceneBlock(prompt string, buf []rune, logPath string) {
	lines := renderLiveLines(currentLiveFrame("", 0))
	for _, line := range lines {
		fmt.Println(line)
	}
	fmt.Print(prompt + string(buf) + "\x1b[K")
}

// overwriteScene 从输入行回到画布顶部，固定覆写 12 行，再准确回到输入行。
func overwriteScene(prompt string, buf []rune, logPath string) {
	lines := renderLiveLines(currentLiveFrame("", 0))
	// 保存输入行光标；每一行都从这个锚点独立定位，任何一行意外换行都不会
	// 累积成下一帧的纵向漂移。刷新过程不输出 CR/LF。
	fmt.Print("\x1b[?25l\x1b7")
	for i, line := range lines {
		fmt.Print("\x1b8\r")
		fmt.Printf("\x1b[%dA", liveSceneRows-i)
		fmt.Print("\x1b[2K" + line)
	}
	fmt.Print("\x1b8\r\x1b[2K" + prompt + string(buf) + "\x1b[K\x1b[?25h")
}
