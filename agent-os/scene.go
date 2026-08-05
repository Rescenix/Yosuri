package main

// scene.go — 楚门世界直播屏（固定高度双栏布局）
//
// 左栏占可用画布 3/4，持续显示开放世界场景；右栏占 1/4，滚动显示日志。
// 整个画布保留终端最右一列，避免 Windows Console/ConPTY 写满一行后自动换行。

import (
	"fmt"
	"strings"
	"time"
)

// liveSceneRows 包含上下边框；输入行始终位于这 12 行之后。
const liveSceneRows = 12

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
	Version    int64    `json:"version"`
	LogLines   []string `json:"-"`
}

var (
	liveFrameMu  = syncMu()
	currentFrame = sceneFrame{RegionName: "出生地", RegionIcon: "🏠", Action: "刚刚醒来", Mood: "(◕‿◕)"}
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
	left, right := sceneColWidthsFor(terminalW)
	bodyRows := liveSceneRows - 2
	sceneLines := leftSceneLines(f)
	logLines := f.LogLines
	if len(logLines) > bodyRows {
		logLines = logLines[len(logLines)-bodyRows:]
	}
	for len(logLines) < bodyRows {
		logLines = append([]string{""}, logLines...)
	}

	title := "楚门世界直播"
	if f.RegionName != "" {
		title += " · " + f.RegionIcon + f.RegionName
	}
	lines := make([]string, 0, liveSceneRows)
	lines = append(lines, sceneSharedTop(title, "日志", left, right))
	for i := 0; i < bodyRows; i++ {
		lc := ""
		if i < len(sceneLines) {
			lc = sceneLines[i]
		}
		lines = append(lines, sceneSharedBody(lc, logLines[i], left, right))
	}
	lines = append(lines, sceneSharedBottom(left, right))
	return lines
}

func leftSceneLines(f sceneFrame) []string {
	bodyRows := liveSceneRows - 2
	lines := make([]string, 0, bodyRows)
	decor := regionDecor(f.RegionKind, f.Seed, f.X, f.Y)
	parts := strings.Split(decor, "\n")
	for i := 0; i < 3; i++ {
		if i < len(parts) {
			lines = append(lines, "  "+parts[i])
		} else {
			lines = append(lines, "  ")
		}
	}

	she := f.Mood
	if f.TravelIcon != "" {
		she = f.TravelIcon + " " + f.Mood
	}
	lines = append(lines, "  "+she+"   ("+fmt.Sprintf("%d,%d", f.X, f.Y)+")")
	lines = append(lines, "  "+f.Action)
	lines = append(lines, "  💗 "+f.Ability)
	if f.Friend != "" {
		lines = append(lines, "  👭 "+f.Friend)
	} else {
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

// drawSceneBlock 首次绘制 12 行直播画布，随后在第 13 行绘制输入提示符。
func drawSceneBlock(prompt string, buf []rune, logPath string) {
	for _, line := range renderLiveLines(currentLiveFrame(logPath, liveSceneRows-2)) {
		fmt.Print("\r\x1b[2K" + line + "\r\n")
	}
	fmt.Print("\r\x1b[2K" + prompt + string(buf) + "\x1b[K")
}

// overwriteScene 从输入行回到画布顶部，固定覆写 12 行，再准确回到输入行。
func overwriteScene(prompt string, buf []rune, logPath string) {
	lines := renderLiveLines(currentLiveFrame(logPath, liveSceneRows-2))
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
