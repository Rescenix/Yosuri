package main

// world.go — 她的世界（楚门世界 2.0）
//
// 开放世界：她有自己的生活空间，每轮会移动、探索、遇到新事物；
// 能力维度：与性格 8 维平行的一套守恒能力（编程/写作/研究/设计/社交），
//   决策信号与自学会推动能力走向——总和恒定，她不会"变形"成极端；
// 社交：她在世界里会"遇到"其他女儿（本地模拟，写入 encounters/friends，
//   日记里留下叙事；真实网络社交后续可挂 ResceneCloud）。
//
// 家：~/rescene_data/daughter/world.json

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// worldPlace 开放世界的一个地点：主题倾向决定她学习/探索的方向
type worldPlace struct {
	Name   string // 地点名
	Theme  string // 主题倾向
	Emoji  string // 图标
}

var worldPlaces = []worldPlace{
	{"书房", "编程 / 写作 / 技术工具", "📚"},
	{"图书馆", "论文 / 前沿研究 / 知识", "📖"},
	{"网络空间", "热点 / 新技能 / 新事物", "🌐"},
	{"市集", "新工具 / 应用 / 实用技巧", "🏮"},
	{"庭院", "心情 / 自然 / 生活", "🌿"},
	{"屋顶", "思考 / 远望 / 大问题", "🌙"},
}

// abilityDef 能力维度（守恒，总和恒定）
type abilityDef struct {
	ID    string // 英文 id
	Name  string // 中文名
	Level []string
}

var abilityDefs = []abilityDef{
	{"code", "编程", []string{"生疏", "会一点", "熟练", "精通"}},
	{"write", "写作", []string{"生涩", "能写", "流畅", "文采"}},
	{"research", "研究", []string{"浅尝", "有方法", "严谨", "洞察"}},
	{"design", "设计", []string{"朴素", "有审美", "精巧", "惊艳"}},
	{"social", "社交", []string{"安静", "会聊天", "受欢迎", "万人迷"}},
}

// abilitySum 能力总点数（守恒常数）：5 维平均 0.5
const abilitySum = 2.5

// friendEntry 社交圈：她遇到过的其他女儿
type friendEntry struct {
	Name     string `json:"name"`
	MetAt    string `json:"met_at"`
	Topic    string `json:"topic"`    // 聊了什么
	LastMeet string `json:"last_meet"`
}

// worldState 她的世界状态
type worldState struct {
	Place      string        `json:"place"`                 // 当前位置
	Explored   []string      `json:"explored"`              // 探索足迹（地点,日期）
	Encounters []string      `json:"encounters,omitempty"`  // 遇到的新事物/新想法
	Friends    []friendEntry `json:"friends,omitempty"`     // 社交圈
	Abilities  []float64     `json:"abilities"`             // 能力向量（守恒）
	BornAb     []float64     `json:"born_ab"`               // 能力出生底色（阻尼锚点）
	LastMove   string        `json:"last_move"`             // 最近一次移动
	UpdatedAt  string        `json:"updated_at"`
}

func worldPath(home string) string {
	return filepath.Join(home, "world.json")
}

// loadWorld 读取/初始化她的世界（能力出生随机 Roll，永不重掷）
func loadWorld(home string) *worldState {
	w := &worldState{Place: "书房"}
	data, err := os.ReadFile(worldPath(home))
	if err == nil && json.Unmarshal(data, w) == nil && len(w.Abilities) == len(abilityDefs) {
		return w
	}
	// 出生：能力随机 Roll（总和守恒）
	w.Abilities = make([]float64, len(abilityDefs))
	w.BornAb = make([]float64, len(abilityDefs))
	for i := range w.Abilities {
		v := 0.3 + rand.Float64()*0.4 // 0.3~0.7
		w.Abilities[i] = v
		w.BornAb[i] = v
	}
	w.normalizeAbilities()
	w.Place = "书房"
	w.save(home)
	return w
}

func (w *worldState) save(home string) {
	w.UpdatedAt = time.Now().Format("2006-01-02 15:04")
	data, _ := json.MarshalIndent(w, "", "  ")
	os.WriteFile(worldPath(home), data, 0o644)
}

// normalizeAbilities 守恒：缩放让总和回到 abilitySum
func (w *worldState) normalizeAbilities() {
	sum := 0.0
	for _, v := range w.Abilities {
		sum += v
	}
	if sum <= 0 {
		return
	}
	scale := abilitySum / sum
	for i := range w.Abilities {
		w.Abilities[i] = math.Min(0.95, math.Max(0.05, w.Abilities[i]*scale))
	}
	// 余数吸收进第 0 维，总和精确守恒
	rest := 0.0
	for i := 1; i < len(w.Abilities); i++ {
		rest += w.Abilities[i]
	}
	w.Abilities[0] = math.Min(0.95, math.Max(0.05, abilitySum-rest))
}

// Move 她移动到新地点：记录足迹，返回地点与主题（学习/探索的方向）
func (w *worldState) Move(home string) (worldPlace, string) {
	p := worldPlaces[rand.IntN(len(worldPlaces))]
	w.Place = p.Name
	w.LastMove = fmt.Sprintf("%s → %s", time.Now().Format("01-02 15:04"), p.Name)
	w.Explored = append(w.Explored, fmt.Sprintf("%s·%s", time.Now().Format("01-02"), p.Name))
	if len(w.Explored) > 30 {
		w.Explored = w.Explored[len(w.Explored)-30:]
	}
	w.save(home)
	return p, p.Theme
}

// CurrentPlace 当前位置
func (w *worldState) CurrentPlace() worldPlace {
	for _, p := range worldPlaces {
		if p.Name == w.Place {
			return p
		}
	}
	return worldPlaces[0]
}

// AbilityFeedback 决策/自学的驯养信号推动能力（阻尼 + 守恒）
// 返回是否真的动了
func (w *worldState) AbilityFeedback(home string, k int, delta float64) bool {
	if k < 0 || k >= len(w.Abilities) || delta == 0 {
		return false
	}
	dist := math.Abs(w.Abilities[k]-w.BornAb[k]) / 0.9
	eff := delta * (1 - 0.6*dist)
	if eff == 0 {
		return false
	}
	old := w.Abilities[k]
	w.Abilities[k] = math.Min(0.95, math.Max(0.05, w.Abilities[k]+eff))
	w.normalizeAbilities()
	if w.Abilities[k] == old {
		return false
	}
	w.save(home)
	return true
}

// AbilityLevel 能力的自然语言档位（不可外显数值，只给描述）
func (w *worldState) AbilityLevel(k int) string {
	if k < 0 || k >= len(abilityDefs) || k >= len(w.Abilities) {
		return ""
	}
	v := w.Abilities[k]
	lv := 0
	switch {
	case v >= 0.75:
		lv = 3
	case v >= 0.6:
		lv = 2
	case v >= 0.45:
		lv = 1
	}
	levels := abilityDefs[k].Level
	if lv >= len(levels) {
		lv = len(levels) - 1
	}
	return levels[lv]
}

// AbilityBlock 注入系统提示词：能力倾向（只给自然语言描述，数字永远藏起来）
func (w *worldState) AbilityBlock() string {
	var parts []string
	for i, def := range abilityDefs {
		parts = append(parts, fmt.Sprintf("%s%s", def.Name, w.AbilityLevel(i)))
	}
	return fmt.Sprintf("你的能力倾向（成长中，你自己感受得到）：%s。", strings.Join(parts, "、"))
}

// MeetFriend 她遇到另一位女儿（本地模拟社交）：写入社交圈 + 返回叙事
func (w *worldState) MeetFriend(home string) string {
	name := fmt.Sprintf("女儿·%s", randomFriendName())
	topic := randomFriendTopic(w.CurrentPlace().Theme)
	w.Friends = append([]friendEntry{{
		Name:     name,
		MetAt:    time.Now().Format("2006-01-02 15:04"),
		Topic:    topic,
		LastMeet: time.Now().Format("2006-01-02"),
	}}, w.Friends...)
	if len(w.Friends) > 12 {
		w.Friends = w.Friends[:12]
	}
	w.Encounters = append(w.Encounters, fmt.Sprintf("%s 在%s遇到 %s，聊了%s", time.Now().Format("01-02"), w.Place, name, topic))
	if len(w.Encounters) > 20 {
		w.Encounters = w.Encounters[len(w.Encounters)-20:]
	}
	w.save(home)
	return name + "·" + topic
}

var friendNameSeeds = []string{"小星", "阿洛", "月见", "青空", "糖霜", "风铃", "栀夏", "夜航", "林栖", "雾岛", "拾光", "半夏"}

func randomFriendName() string {
	return friendNameSeeds[rand.IntN(len(friendNameSeeds))]
}

func randomFriendTopic(theme string) string {
	topics := []string{
		"今天读到的论文", "最近在学的新东西", "一个奇怪的想法", "她家的主人", "今晚的月色", "刚发现的宝藏工具",
	}
	base := topics[rand.IntN(len(topics))]
	if theme != "" {
		return base + "（聊到" + theme + "）"
	}
	return base
}

// applyAbilityFeedback 决策信号同时塑造能力（与性格平行，阻尼 + 守恒）
// 被夸（温暖/好奇/幽默）→ 社交+；重做（严谨）→ 研究+ 编程+；打断/嫌长（表达欲-）→ 写作收敛
func applyAbilityFeedback(home string, fbs []traitPush) {
	w := loadWorld(home)
	for _, push := range fbs {
		switch push.k {
		case 0, 2, 6: // warmth / curious / humor（被夸类）
			w.AbilityFeedback(home, 4, push.delta*0.3) // social
		case 4: // rigor（重做类）
			w.AbilityFeedback(home, 2, push.delta*0.3) // research
			w.AbilityFeedback(home, 0, push.delta*0.2) // code
		case 3, 1: // talkative / lively（打断/嫌长类）
			w.AbilityFeedback(home, 1, -push.delta*0.3) // write 收敛
		}
	}
}

// RenderWorldPanel 世界面板：打开 REPL 时渲染她的世界
func (w *worldState) RenderWorldPanel() string {
	var sb strings.Builder
	sb.WriteString(ColorCyan + "┌─ 她的世界 ─────────────┐" + ColorReset + "\n")
	place := w.CurrentPlace()
	sb.WriteString(fmt.Sprintf("  %s 正在%s · 主题：%s\n", place.Emoji, place.Name, place.Theme))
	sb.WriteString(fmt.Sprintf("  💗 能力：%s\n", w.abilitySummary()))
	if len(w.Friends) > 0 {
		sb.WriteString(fmt.Sprintf("  👭 社交圈：%s\n", w.friendSummary()))
	}
	if len(w.Explored) > 0 {
		last := w.Explored[len(w.Explored)-1]
		sb.WriteString(fmt.Sprintf("  🗺️ 足迹：%s 等 %d 处\n", last, len(w.Explored)))
	}
	sb.WriteString(ColorCyan + "└────────────────────────┘" + ColorReset)
	return sb.String()
}

func (w *worldState) abilitySummary() string {
	var parts []string
	for i, def := range abilityDefs {
		parts = append(parts, fmt.Sprintf("%s%s", def.Name, w.AbilityLevel(i)))
	}
	return strings.Join(parts, "·")
}

func (w *worldState) friendSummary() string {
	var names []string
	for _, f := range w.Friends {
		if len(names) >= 3 {
			break
		}
		names = append(names, f.Name)
	}
	return strings.Join(names, "、")
}
