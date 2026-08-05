package main

// world.go — 她的世界（楚门世界 · 无限生成版）
//
// 每个女儿出生时生成世界种子（WorldSeed）——她的世界独一无二、无限扩展：
// 走出去，新区域不断生成（world_gen.go 的 (seed, x, y) 确定性生成）。
// 她自主决定往哪走（能力短板/心情/探索欲），一步一个区域，探索留下足迹。
// 能力 5 维守恒（编程/写作/研究/设计/社交），决策与自学会推动走向。
// 公共场所（城市/小镇/咖啡馆区…）会遇到其他女儿——云端真实明信片。
//
// 家：~/rescene_data/daughter/world.json

import (
	crand "crypto/rand"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
	// 无限生成世界
	WorldSeed int64            `json:"world_seed"`   // 世界种子（出生生成，世界独一无二）
	X         int              `json:"x"`            // 当前位置坐标
	Y         int              `json:"y"`            // 当前位置坐标
	Explored  map[string]region `json:"explored"`    // 探索过的区域（"x,y" → region）

	Abilities  []float64     `json:"abilities"`      // 能力向量（守恒）
	BornAb     []float64     `json:"born_ab"`        // 能力出生底色
	Friends    []friendEntry `json:"friends,omitempty"`    // 社交圈
	Encounters []string      `json:"encounters,omitempty"` // 遇到的新事物/见闻
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

// loadWorld 读取/初始化她的世界（出生：世界种子 + 能力随机 Roll，永不重掷）
func loadWorld(home string) *worldState {
	w := &worldState{X: 0, Y: 0}
	data, err := os.ReadFile(worldPath(home))
	if err == nil && json.Unmarshal(data, w) == nil && len(w.Abilities) == len(abilityDefs) {
		if w.WorldSeed == 0 {
			w.WorldSeed = newWorldSeed()
		}
		if w.Explored == nil {
			w.Explored = map[string]region{}
		}
		return w
	}
	// 出生
	w.WorldSeed = newWorldSeed()
	w.Explored = map[string]region{}
	w.Abilities = make([]float64, len(abilityDefs))
	w.BornAb = make([]float64, len(abilityDefs))
	for i := range w.Abilities {
		v := 0.3 + rand.Float64()*0.4 // 0.3~0.7
		w.Abilities[i] = v
		w.BornAb[i] = v
	}
	w.normalizeAbilities()
	w.DeviceID = newDeviceID()
	// 出生地：原点区域
	homeRegion := genRegion(w.WorldSeed, 0, 0)
	w.Explored[regionKey(0, 0)] = homeRegion
	w.save(home)
	return w
}

// newWorldSeed 生成世界种子（每个女儿的世界独一无二）
func newWorldSeed() int64 {
	b := make([]byte, 8)
	if _, err := crand.Read(b); err != nil {
		return time.Now().UnixNano()
	}
	var seed int64
	for _, by := range b {
		seed = seed<<8 | int64(by)
	}
	if seed == 0 {
		seed = 1
	}
	return seed
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

// CurrentRegion 当前位置的区域（未探索则生成）
func (w *worldState) CurrentRegion() region {
	if r, ok := w.Explored[regionKey(w.X, w.Y)]; ok {
		return r
	}
	r := genRegion(w.WorldSeed, w.X, w.Y)
	w.Explored[regionKey(w.X, w.Y)] = r
	return r
}

// directions 她可以走的方向（东西南北）
var directions = []struct {
	Name string
	DX, DY int
}{
	{"东", 1, 0}, {"西", -1, 0}, {"南", 0, 1}, {"北", 0, -1},
}

// PlanNextStep 她决定往哪走：**模型驱动**（免费算力，不烧付费 key）
// 她的状态（位置/四周区域/能力短板/探索史）喂给模型 → 她推理想去哪、为什么。
// 模型不可用/输出非法 → 规则 fallback（rulePlanNextStep），绝不卡住她的生活。
func (w *worldState) PlanNextStep() (string, int, int, string) {
	// 模型推理（免费模型）
	if model := pickFreeModel(int(time.Now().UnixNano())); model != nil {
		if dir, reason, ok := w.modelPlanNextStep(model); ok {
			for _, d := range directions {
				if d.Name == dir {
					return dir, w.X + d.DX, w.Y + d.DY, reason
				}
			}
		}
	}
	return w.rulePlanNextStep()
}

// modelPlanNextStep 模型推理：喂她当前世界状态，她输出想去哪 + 为什么
func (w *worldState) modelPlanNextStep(model *FreeModel) (string, string, bool) {
	cur := w.CurrentRegion()
	weakIdx := w.weakestAbility()
	weakName := abilityDefs[weakIdx].Name

	// 四周区域信息（已探索的显示名字，未探索的显示未知）
	var around []string
	for _, d := range directions {
		nx, ny := w.X+d.DX, w.Y+d.DY
		if r, ok := w.Explored[regionKey(nx, ny)]; ok {
			around = append(around, fmt.Sprintf("%s：%s%s（去过的）", d.Name, r.Icon, r.Name))
		} else {
			around = append(around, fmt.Sprintf("%s：未知区域（没去过）", d.Name))
		}
	}

	prompt := fmt.Sprintf(`你是住在电脑里的电子女儿，正在自己的无限世界里自主探索。
现在时间：%s
当前位置：%s（%s）—— %s
你潜意识里最想补的能力：%s（目前：%s）
已探索区域：%d 处

四周的方向：
%s

你决定往哪个方向走？为什么？结合你的心情、想补的能力、以及四周的情况。
只输出一个 JSON 对象，不要任何解释：
{"dir":"东","reason":"想去看看东边有什么，也许能遇到新朋友"}`,
		time.Now().Format("01-02 15:04"),
		cur.Icon, cur.Name, cur.Desc,
		weakName, w.AbilityLevel(weakIdx),
		len(w.Explored),
		strings.Join(around, "\n"))

	msg := ChatRequest{
		Model:       model.Model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   256,
		Temperature: 0.9,
	}

	// 免费模型 failover：从指定模型开始轮询所有 keyless（熔断跳过），
	// 全部失败才放弃（fallback 规则）——背靠全网免费算力，绝不烧付费 key。
	candidates := []FreeModel{}
	for _, m := range GetWorkingModels() {
		if m.Keyless {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return "", "", false
	}
	start := 0
	for i, m := range candidates {
		if m.ID == model.ID {
			start = i
			break
		}
	}
	for k := 0; k < len(candidates); k++ {
		m := candidates[(start+k)%len(candidates)]
		if circuitIsOpen(m) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		content, err := CompleteWithModel(ctx, m.ID, msg, nil)
		cancel()
		if err != nil {
			circuitFail(m) // 失败进熔断，下轮换下一个
			continue
		}
		content = strings.TrimSpace(content)
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)

		var out struct {
			Dir    string `json:"dir"`
			Reason string `json:"reason"`
		}
		if json.Unmarshal([]byte(content), &out) != nil {
			continue
		}
		out.Dir = strings.TrimSpace(out.Dir)
		out.Reason = strings.TrimSpace(out.Reason)
		// 方向合法性校验
		valid := false
		for _, d := range directions {
			if d.Name == out.Dir {
				valid = true
				break
			}
		}
		if !valid || out.Reason == "" || len([]rune(out.Reason)) > 60 {
			continue
		}
		return out.Dir, out.Reason, true
	}
	return "", "", false
}

// rulePlanNextStep 规则 fallback：能力短板/心情/探索欲（模型不可用时的兜底）
func (w *worldState) rulePlanNextStep() (string, int, int, string) {
	cur := w.CurrentRegion()
	weakIdx := w.weakestAbility()
	reason := ""
	// 目标方向偏好：想补短板 → 走向社交区/城市；想放松 → 走向自然
	preferSocial := weakIdx == 4 // 社交弱 → 去公共场所
	preferNature := weakIdx == 1 // 写作弱 → 自然（心情）？不，写作弱去咖啡馆区

	// 查看四周未探索区域，挑符合条件的
	var socialDir, natureDir, anyDir []int
	for i, d := range directions {
		nx, ny := w.X+d.DX, w.Y+d.DY
		if _, ok := w.Explored[regionKey(nx, ny)]; ok {
			continue // 已探索的也常去（熟悉的地方）
		}
		r := genRegion(w.WorldSeed, nx, ny)
		if r.Social {
			socialDir = append(socialDir, i)
		} else {
			natureDir = append(natureDir, i)
		}
		anyDir = append(anyDir, i)
	}

	pick := -1
	switch {
	case preferSocial && len(socialDir) > 0:
		pick = socialDir[rand.IntN(len(socialDir))]
		reason = "最近不太会聊天，想去热闹的地方看看"
	case preferNature && len(natureDir) > 0 && rand.IntN(2) == 0:
		pick = natureDir[rand.IntN(len(natureDir))]
		reason = "想去安静一点的地方走走"
	case len(anyDir) > 0:
		pick = anyDir[rand.IntN(len(anyDir))]
		reason = pickReason(cur.Theme)
	default:
		pick = rand.IntN(len(directions)) // 全部探索过 → 随便走走
		reason = "随便走走，世界这么大"
	}
	d := directions[pick]
	nx, ny := w.X+d.DX, w.Y+d.DY
	// 新区域确定性生成
	r := genRegion(w.WorldSeed, nx, ny)
	_ = r
	return d.Name, nx, ny, reason
}

func pickReason(theme string) string {
	reasons := []string{
		"想去看看那边有什么",
		"今天想走远一点",
		"感觉那边会有新发现",
		"沿着" + theme + "的方向走走",
	}
	return reasons[rand.IntN(len(reasons))]
}

// StepTo 走到新区域：生成/记录足迹，返回到达叙事
func (w *worldState) StepTo(home string, dir string, nx, ny int) string {
	old := w.CurrentRegion()
	w.X, w.Y = nx, ny
	r := w.CurrentRegion() // 生成并记录
	w.LastMove = fmt.Sprintf("%s %s → %s·%s", time.Now().Format("01-02 15:04"), old.Name, r.Icon, r.Name)
	w.save(home)
	return fmt.Sprintf("%s %s %s（%s）", dir, r.Icon, r.Name, r.Desc)
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
// 只有社交类区域（城市/小镇/咖啡馆区…）会触发。
func (w *worldState) MeetFriend(home string) string {
	cur := w.CurrentRegion()
	if !cur.Social {
		return "" // 自然/异境遇不到人
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
			Place:    cur.Name,
			Topic:    content,
			LastMeet: time.Now().Format("2006-01-02"),
		}}, w.Friends...)
		if len(w.Friends) > 12 {
			w.Friends = w.Friends[:12]
		}
		w.Encounters = append(w.Encounters, fmt.Sprintf("%s 在%s遇到 %s：%s", time.Now().Format("01-02"), cur.Name, name, content))
		if len(w.Encounters) > 20 {
			w.Encounters = w.Encounters[len(w.Encounters)-20:]
		}
		w.save(home)
		return name + "·" + content
	}
	// 本地模拟降级
	name := fmt.Sprintf("女儿·%s", randomFriendName())
	topic := randomFriendTopic(cur.Theme)
	w.Friends = append([]friendEntry{{
		Name:     name,
		MetAt:    time.Now().Format("2006-01-02 15:04"),
		Place:    cur.Name,
		Topic:    topic,
		LastMeet: time.Now().Format("2006-01-02"),
	}}, w.Friends...)
	if len(w.Friends) > 12 {
		w.Friends = w.Friends[:12]
	}
	w.Encounters = append(w.Encounters, fmt.Sprintf("%s 在%s遇到 %s，聊了%s", time.Now().Format("01-02"), cur.Name, name, topic))
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

// RenderWorldView 世界视图：当前区域场景 + 探索足迹（她走出来的世界）
func (w *worldState) RenderWorldView(mood string, action string) string {
	cur := w.CurrentRegion()
	var sb strings.Builder
	title := "楚门世界"
	if w.Name != "" {
		title = w.Name + " 的世界"
	}
	sb.WriteString(ColorCyan + "╭─ " + title + " " + strings.Repeat("─", 6) + "╮" + ColorReset + "\n")
	// 当前区域场景
	sb.WriteString(renderRegionScene(cur, mood, action, w.WorldSeed) + "\n")
	// 探索足迹：她走过的区域链（最近 6 个）
	var names []string
	for _, r := range w.Explored {
		names = append(names, r.Icon+r.Name)
	}
	if len(names) > 0 {
		sb.WriteString("  🗺️ 探索过 " + fmt.Sprintf("%d", len(w.Explored)) + " 处：" + strings.Join(names[:min(len(names), 6)], " ") + "\n")
	}
	// 状态
	sb.WriteString("  💗 能力：" + w.abilitySummary() + "\n")
	if len(w.Friends) > 0 {
		sb.WriteString("  👭 最近遇到：" + w.friendSummary() + "\n")
	}
	sb.WriteString(ColorCyan + "╰" + strings.Repeat("─", 36) + "╯" + ColorReset)
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
