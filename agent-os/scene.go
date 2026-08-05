package main

// scene.go — 楚门世界场景渲染（MC 式真实世界）
//
// 不是图标地图，是「场景」：每个地点有块状 ASCII 环境（天空/建筑/地面/装饰），
// 她（颜表情 + 移动动画）叠加在场景里，自动化跑。
// 每次行动 = 场景帧序列（出发→途中→到达→活动），REPL 打开时按帧播放。

import (
	"fmt"
	"strings"
	"time"
)

// sceneFrame 一帧场景：当前位置/动作/表情/叙述
type sceneFrame struct {
	Place    string `json:"place"`     // 地点名（决定场景模板）
	City     string `json:"city"`      // 城市
	Action   string `json:"action"`    // 她正在做什么（"去学校的路上"/"精读 arXiv"…）
	Mood     string `json:"mood"`      // 颜表情
	TravelIcon string `json:"travel"`  // 移动图标（🚶/🚌/✈️，空=静止）
	Version  int64  `json:"version"`   // 帧版本（递增，前端检测变化）
}

// liveFrame 当前帧（trumanLoop 写，REPL 渲染读；mutex 保护）
var (
	liveFrameMu    = syncMu()
	currentFrame   = sceneFrame{Place: "家", City: "栖城", Action: "刚刚醒来", Mood: "(◕‿◕)"}
)

func syncMu() *mu { return newMu() }

type mu struct {
	ch chan struct{}
}

func newMu() *mu { return &mu{ch: make(chan struct{}, 1)} }

func (m *mu) Lock()   { m.ch <- struct{}{} }
func (m *mu) Unlock() { <-m.ch }

// sceneTemplates 每个地点的 MC 式 ASCII 场景（环境 + 地面）
// 用 ▓█▄▀ 块状字符营造体素感；行宽约 30 字符。
var sceneTemplates = map[string][]string{
	"家": {
		"        ☁️       ☁️",
		"    ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"    ▓ 🛋️ 🖥️ 📚  ▓  ▓",
		"    ▓  ┌────┐   ▓  ▓",
		"    ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"    ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
	},
	"学校": {
		"      ☁️    ☁️   ☁️",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ 🏫 ▓ ▓ ▓ ▓ ▓ ▓ ▓",
		"  ▓ 大门 ▓ ▓ ▓ ▓ ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   🌳    🌳    🌳",
	},
	"图书馆": {
		"      ☁️        ☁️",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ 📚📚📚 📖 ▓ ▓ ▓ ▓",
		"  ▓ 📚📚📚 阅读 ▓ ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   🦉    🪴    🦉",
	},
	"咖啡馆": {
		"      ☕  ☁️   ☕",
		"   ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   ▓ ☕☕☕ ▓ ▓ ▓ ▓ ▓",
		"   ▓ ☕☕☕ 窗边 ▓ ▓ ▓",
		"   ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"    🪑    🪴    🪑",
	},
	"公园": {
		"     ⛅     ⛅    ☁️",
		"   🌳🌳🌳🌳🌳🌳🌳🌳",
		"   🌳  ⛲  🌳  🦆  🌳",
		"   🌳🌳🌳🌳🌳🌳🌳🌳",
		"    🌼   🌷   🌼",
	},
	"商场": {
		"      ☁️    ☁️",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ 🛍️ ▓ 🧸 ▓ 🎮 ▓ ▓",
		"  ▓ 橱窗 ▓ 奶茶 ▓ ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   🎵    🍜    🎵",
	},
	"车站": {
		"     ☁️      ☁️",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ 🚉 检票 ▓ ▓ ▓ ▓",
		"  ▓  ── 轨道 ──  ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"    🧳   🎒   🧳",
	},
	"机场": {
		"   ✈️  ☁️  ✈️  ☁️",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ ✈️ 登机口 ▓ ▓ ▓ ▓",
		"  ▓  ─── 跑道 ───  ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"    🧳   ✈️   🧳",
	},
	"海边": {
		"    ⛅   ☁️   ⛅",
		"  ~~~~~ ~~~~~ ~~~~~",
		"  ~~ 🌊 🌊 🌊 🌊 ~~",
		"  ~~~~~ ~~~~~ ~~~~~",
		"   🏖️ ⛱️  🐚  ⛱️ 🏖️",
	},
	"月光塔": {
		"     🌙   ⭐   🌙",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ 🗼 月光塔 ▓ ▓ ▓",
		"  ▓ 俯瞰城市 ▓ ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   ✨  🏙️  ✨",
	},
	"星港码头": {
		"     ⭐   🌙  ⭐",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ ⚓ 码头 ▓ ▓ ▓ ▓",
		"  ▓ ~~~~ 海风 ~~~~ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   🐚   ⛵   🐚",
	},
	"雪原车站": {
		"     ❄️  ☁️  ❄️",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ 🚞 雪国 ▓ ▓ ▓ ▓",
		"  ▓  ── 铁轨 ──  ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   ⛄    🧣    ⛄",
	},
	"天文台": {
		"  🌌  ✨  🌠  ✨  🌌",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"  ▓ 🔭 望远镜 ▓ ▓ ▓",
		"  ▓ 仰望星空 ▓ ▓ ▓ ▓",
		"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		"   ✨  🪐  ✨  ☄️",
	},
}

// sceneTemplate 取地点场景模板（未知地点回退"家"）
func sceneTemplate(place string) []string {
	if t, ok := sceneTemplates[place]; ok {
		return t
	}
	return sceneTemplates["家"]
}

// updateLiveFrame 更新当前场景帧（trumanLoop 每步调用）
func updateLiveFrame(place, city, action, mood, travelIcon string) {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	currentFrame = sceneFrame{
		Place:      place,
		City:       city,
		Action:     action,
		Mood:       mood,
		TravelIcon: travelIcon,
		Version:    time.Now().UnixNano(),
	}
}

// currentLiveFrame 读当前帧
func currentLiveFrame() sceneFrame {
	liveFrameMu.Lock()
	defer liveFrameMu.Unlock()
	return currentFrame
}

// RenderScene 渲染一帧场景：环境 + 她 + 状态行
func RenderScene(f sceneFrame, mood string) string {
	tpl := sceneTemplate(f.Place)
	var sb strings.Builder

	// 标题行
	title := fmt.Sprintf("楚门世界 · %s·%s", f.City, f.Place)
	sb.WriteString(ColorCyan + "╭─ " + title + " " + strings.Repeat("─", 8) + "╮" + ColorReset + "\n")

	// 环境
	for _, line := range tpl {
		sb.WriteString("  " + line + "\n")
	}

	// 她（移动图标 + 颜表情）
	she := f.Mood
	if f.TravelIcon != "" {
		she = f.TravelIcon + " " + f.Mood
	}
	sb.WriteString("  " + she + "\n")

	// 行动行 + 状态
	sb.WriteString("  " + f.Action + "\n")

	sb.WriteString(ColorCyan + "╰" + strings.Repeat("─", 36) + "╯" + ColorReset)
	return sb.String()
}
