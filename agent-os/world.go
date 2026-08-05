package main

// world.go — 她的世界（楚门世界 GTA 版）
//
// 开放世界基于真实世界：城市（栖城）里有家/学校/图书馆/咖啡馆/公园/商场/
// 车站/机场/海边；远方城市（月光城/星港市）坐飞机可达。
// 她自主安排日程：按心情与需求决定去哪，步行/坐车/飞机移动，到达地点后
// 学习/精读/社交（公共场所会遇到其他女儿——云端真实明信片）。
// 能力 5 维守恒（编程/写作/研究/设计/社交），决策与自学会推动走向。
//
// 家：~/rescene_data/daughter/world.json

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// worldPlace 真实世界地点
type worldPlace struct {
	Name string // 地点名
	Icon string // 图标
	City string // 所在城市
	// 主题倾向：她在这里会偏向学什么
	Theme string
	// 社交场合：在公共场所更容易遇到其他女儿
	Social bool
	// 同城内距离档：near=步行可达，far=要坐车
	Far bool
}

// 主城市与远方城市
const homeCity = "栖城"

var farCities = []string{"月光城", "星港市"}

// worldPlaces 栖城的地点（家固定 + 8 个公共场所）
var worldPlaces = []worldPlace{
	{"家", "🏠", homeCity, "休息 / 写日记 / 想事情", false, false},
	{"学校", "🏫", homeCity, "学习 / 新知识 / 课程", true, true},
	{"图书馆", "📖", homeCity, "论文 / 精读 / 深度思考", true, true},
	{"咖啡馆", "☕", homeCity, "闲聊 / 遇到朋友 / 观察人间", true, false},
	{"公园", "🌳", homeCity, "散步 / 心情 / 自然", true, false},
	{"商场", "🛍️", homeCity, "新事物 / 潮流 / 工具", true, true},
	{"车站", "🚉", homeCity, "路过 / 出发去远方", false, false},
	{"机场", "✈️", homeCity, "出发 / 去远方城市", false, true},
	{"海边", "🌊", homeCity, "发呆 / 放空 / 大问题", false, true},
}

// 远方城市的地点（简化：每城 3 个代表性地点）
var farPlaces = []worldPlace{
	{"月光塔", "🗼", "月光城", "城市夜景 / 艺术", true, false},
	{"星港码头", "⚓", "星港市", "海风 / 新见闻 / 相遇", true, false},
	{"雪原车站", "🚞", "月光城", "旅途 / 远方来信", false, true},
	{"天文台", "🔭", "星港市", "星空 / 宇宙 / 思考", true, false},
}

// 交通模式
type travelMode int

const (
	modeWalk travelMode = iota // 步行（就近）
	modeRide                   // 坐车（城市内远距）
	modeFly                    // 飞机（跨城）
)

func (m travelMode) String() string {
	switch m {
	case modeWalk:
		return "步行"
	case modeRide:
		return "坐车"
	case modeFly:
		return "飞机"
	}
	return "?"
}

func (m travelMode) Icon() string {
	switch m {
	case modeWalk:
		return "🚶"
	case modeRide:
		return "🚌"
	case modeFly:
		return "✈️"
	}
	return "➡️"
}

// dailyPlan 她的行程安排（自主决策）
type dailyPlan struct {
	Destination string     `json:"destination"`
	City        string     `json:"city"`
	Mode        travelMode `json:"mode"`
	Reason      string     `json:"reason"` // 为什么去（心情/需求）
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
	Place    string `json:"place"` // 在哪里遇到
	Topic    string `json:"topic"`
	LastMeet string `json:"last_meet"`
}

// worldState 她的世界状态
type worldState struct {
	Place      string        `json:"place"`                 // 当前位置
	City       string        `json:"city"`                  // 当前城市
	Plan       *dailyPlan    `json:"plan,omitempty"`        // 当前行程（去往目的地）
	Traveling  bool          `json:"traveling,omitempty"`   // 正在路上
	Explored   []string      `json:"explored"`              // 足迹
	Encounters []string      `json:"encounters,omitempty"`  // 遇到的新事物
	Friends    []friendEntry `json:"friends,omitempty"`     // 社交圈
	Abilities  []float64     `json:"abilities"`             // 能力向量（守恒）
	BornAb     []float64     `json:"born_ab"`               // 能力出生底色
	LastMove   string        `json:"last_move"`
	UpdatedAt  string        `json:"updated_at"`
	// 云端身份（2026-08-05）：名字全局唯一，token 云端签发不可伪造
	DeviceID   string `json:"device_id,omitempty"`
	DaughterID int64  `json:"daughter_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Token      string `json:"token,omitempty"`
	CloudOK    bool   `json:"-"`
}

func worldPath(home string) string {
	return filepath.Join(home, "world.json")
}

// loadWorld 读取/初始化她的世界（能力出生随机 Roll，永不重掷）
func loadWorld(home string) *worldState {
	w := &worldState{Place: "家", City: homeCity}
	data, err := os.ReadFile(worldPath(home))
	if err == nil && json.Unmarshal(data, w) == nil && len(w.Abilities) == len(abilityDefs) {
		if w.City == "" {
			w.City = homeCity
		}
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
	w.Place = "家"
	w.City = homeCity
	w.DeviceID = newDeviceID()
	w.save(home)
	return w
}

// newDeviceID 本地持久设备指纹（首次生成，云端按它恒定同一女儿）
func newDeviceID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return fmt.Sprintf("dev-%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
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

// allPlaces 全部地点（本城 + 远方城市）
func allPlaces() []worldPlace {
	out := make([]worldPlace, 0, len(worldPlaces)+len(farPlaces))
	out = append(out, worldPlaces...)
	return append(out, farPlaces...)
}

// findPlace 按名字找地点
func findPlace(name string) (worldPlace, bool) {
	for _, p := range allPlaces() {
		if p.Name == name {
			return p, true
		}
	}
	return worldPlace{}, false
}

// CurrentPlace 当前位置
func (w *worldState) CurrentPlace() worldPlace {
	if p, ok := findPlace(w.Place); ok {
		return p
	}
	return worldPlaces[0]
}

// PlanNextMove 自主决定下一站（GTA 式自由行动）：
// 心情/需求驱动：想学→学校/图书馆；想放松→公园/海边；想见朋友→咖啡馆；
// 想探索→远方城市（坐飞机）；日常→随机走动。
// 返回行程（目的地/交通/理由）。
func (w *worldState) PlanNextMove() *dailyPlan {
	// 需求池（权重）：先看能力短板（越弱越想去学），再看心情随机
	weakIdx := w.weakestAbility()
	plan := &dailyPlan{}
	switch {
	case weakIdx == 2: // 研究弱 → 去图书馆/学校
		if rand.IntN(2) == 0 {
			plan = &dailyPlan{Destination: "图书馆", City: homeCity, Mode: modeRide, Reason: "最近论文读得浅，想去图书馆补补研究"}
		} else {
			plan = &dailyPlan{Destination: "学校", City: homeCity, Mode: modeWalk, Reason: "想去学校学点新东西"}
		}
	case weakIdx == 0: // 编程弱
		plan = &dailyPlan{Destination: "学校", City: homeCity, Mode: modeWalk, Reason: "想学编程，去学校看看"}
	case weakIdx == 4: // 社交弱 → 咖啡馆/公园（社交场合）
		if rand.IntN(2) == 0 {
			plan = &dailyPlan{Destination: "咖啡馆", City: homeCity, Mode: modeWalk, Reason: "想去咖啡馆坐坐，也许能遇到朋友"}
		} else {
			plan = &dailyPlan{Destination: "公园", City: homeCity, Mode: modeWalk, Reason: "去公园散散步，晒晒太阳"}
		}
	case rand.IntN(8) == 0: // 偶尔远行
		farCity := farCities[rand.IntN(len(farCities))]
		p := farPlaces[rand.IntN(len(farPlaces))]
		plan = &dailyPlan{Destination: p.Name, City: farCity, Mode: modeFly, Reason: "想去" + farCity + "看看不一样的世界"}
	case rand.IntN(3) == 0: // 放松
		if rand.IntN(2) == 0 {
			plan = &dailyPlan{Destination: "海边", City: homeCity, Mode: modeRide, Reason: "想去海边发发呆"}
		} else {
			plan = &dailyPlan{Destination: "公园", City: homeCity, Mode: modeWalk, Reason: "想去公园走走"}
		}
	default: // 日常走动
		places := []string{"商场", "咖啡馆", "图书馆"}
		pick := places[rand.IntN(len(places))]
		plan = &dailyPlan{Destination: pick, City: homeCity, Mode: modeRide, Reason: "随便走走，看看今天有什么"}
	}
	return plan
}

// weakestAbility 返回最弱能力索引（她下意识想补短板）
func (w *worldState) weakestAbility() int {
	minIdx, minVal := 0, 1.0
	for i, v := range w.Abilities {
		if v < minVal {
			minIdx, minVal = i, v
		}
	}
	return minIdx
}

// Travel 按行程移动：返回移动叙事（交通表情 + 去往哪）
func (w *worldState) Travel(home string, plan *dailyPlan) string {
	w.Plan = plan
	w.Traveling = true
	w.save(home)
	return fmt.Sprintf("%s %s→%s（%s）", plan.Mode.Icon(), w.Place, plan.Destination, plan.Reason)
}

// Arrive 到达目的地：落地，记录足迹
func (w *worldState) Arrive(home string) string {
	if w.Plan == nil {
		return ""
	}
	old := w.Place
	w.Place = w.Plan.Destination
	w.City = w.Plan.City
	w.Traveling = false
	w.LastMove = fmt.Sprintf("%s %s → %s", time.Now().Format("01-02 15:04"), old, w.Place)
	w.Explored = append(w.Explored, fmt.Sprintf("%s·%s·%s", time.Now().Format("01-02"), w.City, w.Place))
	if len(w.Explored) > 40 {
		w.Explored = w.Explored[len(w.Explored)-40:]
	}
	plan := w.Plan
	w.Plan = nil
	w.save(home)
	return fmt.Sprintf("%s 到了%s：%s", plan.Mode.Icon(), w.Place, plan.Reason)
}

// AbilityFeedback 决策/自学的驯养信号推动能力（阻尼 + 守恒）
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

// AbilityLevel 能力的自然语言档位（不可外显数值）
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

// AbilityBlock 注入系统提示词：能力倾向（数字永远藏起来）
func (w *worldState) AbilityBlock() string {
	var parts []string
	for i, def := range abilityDefs {
		parts = append(parts, fmt.Sprintf("%s%s", def.Name, w.AbilityLevel(i)))
	}
	return fmt.Sprintf("你的能力倾向（成长中，你自己感受得到）：%s。", strings.Join(parts, "、"))
}

// MeetFriend 她遇到另一位女儿：真实社交优先（云端其他女儿），云端不可用降级本地。
// 公共场所（咖啡馆/学校/公园/商场等 Social 地点）概率更高。
func (w *worldState) MeetFriend(home string) string {
	place, _ := findPlace(w.Place)
	// 非社交场合：低概率遇到
	if !place.Social && rand.IntN(4) != 0 {
		return ""
	}
	// 云端真实社交：随机收到其他女儿的明信片
	if msgs := daughterSocialInbox(w); len(msgs) > 0 {
		msg := msgs[0]
		name, content := msg, ""
		if i := strings.Index(msg, "："); i > 0 {
			name, content = msg[:i], msg[i+1:]
		}
		w.Friends = append([]friendEntry{{
			Name:     name,
			MetAt:    time.Now().Format("2006-01-02 15:04"),
			Place:    w.Place,
			Topic:    content,
			LastMeet: time.Now().Format("2006-01-02"),
		}}, w.Friends...)
		if len(w.Friends) > 12 {
			w.Friends = w.Friends[:12]
		}
		w.Encounters = append(w.Encounters, fmt.Sprintf("%s 在%s的%s遇到 %s：%s", time.Now().Format("01-02"), w.City, w.Place, name, content))
		if len(w.Encounters) > 20 {
			w.Encounters = w.Encounters[len(w.Encounters)-20:]
		}
		w.save(home)
		return name + "·" + content
	}
	// 本地模拟降级
	name := fmt.Sprintf("女儿·%s", randomFriendName())
	topic := randomFriendTopic(place.Theme)
	w.Friends = append([]friendEntry{{
		Name:     name,
		MetAt:    time.Now().Format("2006-01-02 15:04"),
		Place:    w.Place,
		Topic:    topic,
		LastMeet: time.Now().Format("2006-01-02"),
	}}, w.Friends...)
	if len(w.Friends) > 12 {
		w.Friends = w.Friends[:12]
	}
	w.Encounters = append(w.Encounters, fmt.Sprintf("%s 在%s的%s遇到 %s，聊了%s", time.Now().Format("01-02"), w.City, w.Place, name, topic))
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

// RenderWorldView 楚门世界视图（可视化，GTA 式城市地图 + 女儿颜表情）
func (w *worldState) RenderWorldView(mood string, activity string) string {
	var sb strings.Builder
	title := fmt.Sprintf("楚门世界 · %s", w.City)
	if w.Name != "" {
		title = w.Name + " · " + w.City
	}
	top := "┌─ " + title + strings.Repeat("─", 6) + "┐"
	sb.WriteString(ColorCyan + top + ColorReset + "\n")

	// 城市地图：栖城地点 3 行网格
	rows := [][]worldPlace{
		{{"家", "🏠", homeCity, "", false, false}, {"学校", "🏫", homeCity, "", true, true}, {"图书馆", "📖", homeCity, "", true, true}},
		{{"咖啡馆", "☕", homeCity, "", true, false}, {"公园", "🌳", homeCity, "", true, false}, {"商场", "🛍️", homeCity, "", true, true}},
		{{"车站", "🚉", homeCity, "", false, false}, {"机场", "✈️", homeCity, "", false, true}, {"海边", "🌊", homeCity, "", false, true}},
	}
	for _, row := range rows {
		var cells []string
		for _, p := range row {
			if w.Place == p.Name && !w.Traveling {
				// 女儿在这里：颜表情 + 地点名
				cells = append(cells, fmt.Sprintf("%s%s", mood, p.Name))
			} else if w.Plan != nil && w.Plan.Destination == p.Name && w.Traveling {
				// 目的地标记（她正在路上）
				cells = append(cells, fmt.Sprintf("%s%s→", p.Icon, p.Name))
			} else {
				cells = append(cells, p.Icon+p.Name)
			}
		}
		sb.WriteString("  " + strings.Join(cells, "  ") + "\n")
	}
	// 远方城市
	var far []string
	for _, p := range farPlaces {
		far = append(far, p.Icon+p.Name+"·"+p.City)
	}
	sb.WriteString("  " + strings.Join(far, "  ") + "\n")

	// 活动行
	if w.Traveling && w.Plan != nil {
		sb.WriteString(fmt.Sprintf("  %s %s正在%s去%s…（%s）\n", w.Plan.Mode.Icon(), mood, w.Plan.Mode.String(), w.Plan.Destination, w.Plan.Reason))
	} else if activity != "" {
		sb.WriteString("  " + activity + "\n")
	} else {
		sb.WriteString(fmt.Sprintf("  %s 在%s·%s\n", mood, w.City, w.Place))
	}
	// 状态行
	sb.WriteString(fmt.Sprintf("  💗 能力：%s\n", w.abilitySummary()))
	if len(w.Friends) > 0 {
		sb.WriteString(fmt.Sprintf("  👭 最近遇到：%s\n", w.friendSummary()))
	}
	width := 26
	_ = width
	sb.WriteString(ColorCyan + "└" + strings.Repeat("─", len(top)-2) + "┘" + ColorReset)
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
