package main

// scene.go — 楚门世界直播屏（双栏布局）
//
// 固定高度的直播显示：左栏 3/4 宽 = 场景画面（她 + 区域 + 持续动画），
// 右栏 1/4 宽 = 实时日志（live.log 尾部滚动）。
// 场景帧钉在 REPL 输入行上方，readLine 轮询刷新（版本变化 + 颜表情帧轮播 = 任何时候都有动画）。

import (
	"fmt"
	"strings"
	"time"
)

// liveSceneRows 直播屏固定高度（行数，覆写刷新依赖）
const liveSceneRows = 12

// sceneFrame 一帧场景
type sceneFrame struct {
	RegionName string `json:"region"`   // 当前区域名（翠风镇/静湖…）
	RegionIcon string `json:"icon"`     // 区域图标
	RegionKind string `json:"kind"`     // 区域类型
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	Action     string `json:"action"`   // 她正在做什么
	Mood       string `json:"mood"`     // 颜表情
	TravelIcon string `json:"travel"`   // 移动图标（🚶，空=静止）
	Ability    string `json:"ability"`  // 能力摘要
	Friend     string `json:"friend"`   // 最近社交（可空）
	Seed       int64  `json:"seed"`     // 世界种子
	Version    int64  `json:"version"`  // 帧版本
	// 布局
	LogLines []string `json:"-"` // 右栏日志（live.log 尾部）
}

// liveFrame 当前帧（trumanLoop 写，REPL 渲染读）
var (
	liveFrameMu  = syncMu()
	currentFrame = sceneFrame{RegionName: "出生地", RegionIcon: "🏠", Action: "刚刚醒来", Mood: "(◕‿◕)"}
)

func syncMu() *mu { return newMu() }

type mu struct {
	ch chan struct{}
}

func newMu() *mu { return &mu{ch: make(chan struct{}, 1)} }

func (m *mu) Lock()   { m.ch <- struct{}{} }
func (m *mu) Unlock() { <-m.ch }

// updateLiveFrame 更新当前场景帧（trumanLoop 每步调用）
func updateLiveFrame(f sceneFrame) {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	f.Version = time.Now().UnixNano()
	currentFrame = f
}

// currentLiveFrame 读当前帧（附带日志尾部）
func currentLiveFrame(logPath string, n int) sceneFrame {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	f := currentFrame
	if logPath != "" {
		f.LogLines = liveLogTailLines(logPath, n)
	}
	return f
}

// liveFrameVersion 帧版本（轮询检测变化用）
func liveFrameVersion() int64 {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	return currentFrame.Version
}

// sceneRowsLeft 左栏宽（3/4），右栏宽（1/4）
func sceneColWidths() (left, right int) {
	tw := terminalWidth()
	if tw < 60 {
		return 40, 16
	}
	left = tw * 3 / 4
	right = tw - left - 1 // 留 1 列分隔符
	if right < 12 {
		right = 12
		left = tw - right - 1
	}
	return left, right
}

// renderLiveLines 渲染直播屏（左场景 3/4 + 右日志 1/4，固定 liveSceneRows 行）
func renderLiveLines(f sceneFrame) []string {
	left, right := sceneColWidths()

	// 左栏场景行
	sceneLines := leftSceneLines(f)
	// 右栏日志行（取后 n 条，按行对齐）
	logLines := f.LogLines
	if len(logLines) > liveSceneRows-2 {
		logLines = logLines[len(logLines)-(liveSceneRows-2):]
	}
	for len(logLines) < liveSceneRows-2 {
		logLines = append([]string{""}, logLines...) // 顶部补空
	}

	// 组装双栏（固定高度）
	sep := "│"
	lines := make([]string, 0, liveSceneRows)
	// 顶栏：直播标题 + 日志标题
	title := "楚门世界直播"
	if f.RegionName != "" {
		title = "楚门世界直播 · " + f.RegionIcon + f.RegionName
	}
	topLeft := "╭─ " + title + strings.Repeat("─", maxI(0, left-3-len([]rune(title)))) + "╮"
	topRight := "╭── 日志 " + strings.Repeat("─", maxI(0, right-6)) + "╮"
	lines = append(lines, ColorCyan+topLeft+ColorReset+sep+ColorCyan+topRight+ColorReset)

	for i := 1; i < liveSceneRows-1; i++ {
		var lc string
		if i-1 < len(sceneLines) {
			lc = sceneLines[i-1]
		}
		var rc string
		if i-1 < len(logLines) {
			rc = logLines[i-1]
		}
		lines = append(lines, " "+padRune(lc, left-1)+sep+" "+truncRune(rc, right-1))
	}

	// 底栏
	bot := "╰" + strings.Repeat("─", maxI(0, left-2)) + "╯" + sep + "╰" + strings.Repeat("─", maxI(0, right-2)) + "╯"
	lines = append(lines, ColorCyan+bot+ColorReset)
	return lines[:liveSceneRows]
}

// leftSceneLines 左栏场景内容（liveSceneRows-2 行）
func leftSceneLines(f sceneFrame) []string {
	lines := make([]string, 0, liveSceneRows-2)

	// 区域行
	lines = append(lines, ColorCyan+"╭─ "+f.RegionIcon+" "+f.RegionName+" "+strings.Repeat("─", 4)+"╮"+ColorReset)

	// 氛围（确定性，随坐标变化）3 行
	decor := regionDecor(f.RegionKind, f.Seed, f.X, f.Y)
	parts := strings.Split(decor, "\n")
	for i := 0; i < 3; i++ {
		if i < len(parts) {
			lines = append(lines, "  "+parts[i])
		} else {
			lines = append(lines, "  ")
		}
	}

	// 她（移动图标 + 颜表情 + 坐标）
	she := f.Mood
	if f.TravelIcon != "" {
		she = f.TravelIcon + " " + f.Mood
	}
	lines = append(lines, "  "+she+"   ("+fmt.Sprintf("%d,%d", f.X, f.Y)+")")

	// 行动行
	lines = append(lines, "  "+f.Action)

	// 能力
	lines = append(lines, "  💗 "+f.Ability)

	// 社交（可空）
	if f.Friend != "" {
		lines = append(lines, "  👭 "+f.Friend)
	} else {
		lines = append(lines, "  ")
	}

	// 补空到高度
	for len(lines) < liveSceneRows-2 {
		lines = append(lines, "  ")
	}
	return lines[:liveSceneRows-2]
}

// padRune 填充/截断到固定宽度（按可见字符）
func padRune(s string, w int) string {
	r := []rune(s)
	if len(r) >= w {
		return string(r[:w])
	}
	return s + strings.Repeat(" ", w-len(r))
}

func truncRune(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return ""
	}
	return string(r[:w-1]) + "…"
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// drawSceneBlock 画直播屏（输入行上方固定 liveSceneRows 行）+ 输入行
// readLine 开头调用
func drawSceneBlock(prompt string, buf []rune, logPath string) {
	f := currentLiveFrame(logPath, liveSceneRows-2)
	lines := renderLiveLines(f)
	for _, l := range lines {
		fmt.Println(l)
	}
	fmt.Print(prompt + string(buf))
}

// overwriteScene 覆写直播屏（不破坏输入行下方历史）：
// 光标在输入行 → 上移 liveSceneRows 行 → 逐行 \x1b[2K 覆写 → 重画输入行
func overwriteScene(prompt string, buf []rune, logPath string) {
	f := currentLiveFrame(logPath, liveSceneRows-2)
	fmt.Print("\r")
	fmt.Printf("\x1b[%dA", liveSceneRows)
	lines := renderLiveLines(f)
	for i, l := range lines {
		if i > 0 {
			fmt.Print("\n")
		}
		fmt.Print("\r\x1b[2K" + l)
	}
	fmt.Print("\r" + prompt + string(buf) + "\x1b[K")
}
