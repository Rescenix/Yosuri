package main

// scene.go — 楚门世界实时直播场景
//
// 不是静态地图：她在你眼前跑。场景帧（固定行数）钉在 REPL 输入行上方，
// trumanLoop 每步更新 liveFrame，readLine 空闲时轮询变化 → 覆写刷新场景区。
// 场景 = 当前区域氛围 + 她（颜表情/移动图标） + 行动行 + 状态行。

import (
	"fmt"
	"strings"
	"time"
)

// liveSceneRows 场景区固定行数（覆写刷新依赖固定行数）
const liveSceneRows = 9

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
	Ability    string `json:"ability"`  // 能力摘要（状态行）
	Friend     string `json:"friend"`   // 最近社交（可空）
	Seed       int64  `json:"seed"`     // 世界种子（区域氛围确定性）
	Version    int64  `json:"version"`  // 帧版本（递增，前端检测变化）
}

// liveFrame 当前帧（trumanLoop 写，REPL 渲染读）
var (
	liveFrameMu    = syncMu()
	currentFrame   = sceneFrame{RegionName: "出生地", RegionIcon: "🏠", Action: "刚刚醒来", Mood: "(◕‿◕)"}
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

// currentLiveFrame 读当前帧
func currentLiveFrame() sceneFrame {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	return currentFrame
}

// liveFrameVersion 帧版本（轮询检测变化用）
func liveFrameVersion() int64 {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	return currentFrame.Version
}

// drawSceneBlock 画场景区（输入行上方固定 liveSceneRows 行）+ 输入行
// readLine 开头调用：场景区在输入行上方，向下打印
func drawSceneBlock(prompt string, buf []rune) {
	lines := renderLiveLines(currentLiveFrame())
	for _, l := range lines {
		fmt.Println(l)
	}
	fmt.Print(prompt + string(buf))
}

// overwriteScene 覆写场景区（不破坏输入行下方历史）：
// 光标在输入行 → 上移 liveSceneRows 行到场景区首行 → 逐行 \x1b[2K 覆写 → 重画输入行
func overwriteScene(prompt string, buf []rune) {
	fmt.Print("\r")
	fmt.Printf("\x1b[%dA", liveSceneRows)
	lines := renderLiveLines(currentLiveFrame())
	for i, l := range lines {
		if i > 0 {
			fmt.Print("\n")
		}
		fmt.Print("\r\x1b[2K" + l)
	}
	// 光标在最后一行（= 输入行位置），重画输入行
	fmt.Print("\r" + prompt + string(buf) + "\x1b[K")
}

// renderLiveLines 渲染固定行数的场景帧（不足补空行）
func renderLiveLines(f sceneFrame) []string {
	lines := make([]string, 0, liveSceneRows)

	// 1 标题
	title := "楚门世界"
	lines = append(lines, ColorCyan+"╭─ "+title+" "+strings.Repeat("─", 8)+"╮"+ColorReset)

	// 2 区域行
	lines = append(lines, ColorCyan+"╭─ "+f.RegionIcon+" "+f.RegionName+" "+strings.Repeat("─", 6)+"╮"+ColorReset)

	// 3-5 区域氛围（确定性，随坐标变化）
	decor := regionDecor(f.RegionKind, f.Seed, f.X, f.Y)
	parts := strings.Split(decor, "\n")
	for i := 0; i < 3; i++ {
		if i < len(parts) {
			lines = append(lines, "  "+parts[i])
		} else {
			lines = append(lines, "  ")
		}
	}

	// 6 她（移动图标 + 颜表情）
	she := f.Mood
	if f.TravelIcon != "" {
		she = f.TravelIcon + " " + f.Mood
	}
	lines = append(lines, "  "+she)

	// 7 行动行
	lines = append(lines, "  "+f.Action)

	// 8 能力
	lines = append(lines, "  💗 能力："+f.Ability)

	// 9 社交（可空）+ 底线
	friendLine := "  👭 "+f.Friend
	if f.Friend == "" {
		friendLine = "  "
	}
	lines = append(lines, friendLine+ColorCyan+"╰"+strings.Repeat("─", 34)+"╯"+ColorReset)

	// 固定行数
	for len(lines) < liveSceneRows {
		lines = append(lines, "  ")
	}
	return lines[:liveSceneRows]
}
