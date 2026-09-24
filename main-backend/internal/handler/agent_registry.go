package handler

// agent_registry.go —— 多 Agent 角色卡注册表。
//
// 一个用户可建多个 Agent，每个 Agent 有独立的：
//   - 角色卡（人设文案 persona：自称/语气/性格/忌讳）
//   - 头像（本地 base64 dataURL，落 ~/rescene_data/agents/<id>/avatar）
//   - 私有记忆（~/rescene_data/agents/<id>/memory/，见 memorydir.agent_memory）
//
// 通用记忆（~/rescene_data/memory/）所有 Agent 共享一份，不复制。
//
// 注册表本体是 ~/rescene_data/agents.json（一个 JSON 数组）。头像和记忆走
// 各自的文件，注册表里只存 id 与元信息，避免把几 MB base64 塞进索引。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backend/internal/memorydir"
)

// AgentStats 人物参数：角色在世界里「处于什么状态」的数值。
// 与 persona（角色是谁）不同，这些值会随剧情涨掉——受伤掉 HP、升级涨
// 等级、买到装备、好感随互动变化。战斗引擎直接读它们。
// 全 0 视为「未设定」：不显示血条、不进战斗，纯聊天 RP 不受影响。
// 视觉沿用星迹风格：HP 条 + 护盾覆盖层 + MP 条 + 印记图标（只抄表现，不抄逻辑）。
type AgentStats struct {
	Level     int      `json:"level,omitempty"`
	Exp       int      `json:"exp,omitempty"` // 当前经验（升级阈值 = level*100，满级封顶 99）
	HP        int      `json:"hp,omitempty"`
	MaxHP     int      `json:"maxHp,omitempty"`
	MP        int      `json:"mp,omitempty"`
	MaxMP     int      `json:"maxMp,omitempty"`
	Shield    int      `json:"shield,omitempty"` // 护盾值（盖在 HP 条上的浅色层）
	ATK       int      `json:"atk,omitempty"`
	DEF       int      `json:"def,omitempty"`
	SPD       int      `json:"spd,omitempty"`
	MAG       int      `json:"mag,omitempty"`
	Element   string   `json:"element,omitempty"` // 元素（fire/water/…，决定印记图标颜色）
	Gold      int      `json:"gold,omitempty"`
	Affection int      `json:"affection,omitempty"` // 对主角好感 0-100
	Equipment []string `json:"equipment,omitempty"`
	Title     string   `json:"title,omitempty"` // 称号/身份（"银剑骑士"）
	// Marks 印记/效果（盾/中毒/印记…）：name + value，面板渲染成图标行。
	Marks []CustomStat `json:"marks,omitempty"`
	// Inventory 背包：战斗掉落/剧情获得的物品（name + 数量）。
	Inventory []InvItem `json:"inventory,omitempty"`
	// Custom 自定义参数：名字任意（灵力/san值/体力/声望…），值统一存字符串，
	// 数值比较时尽力转 int/float，转不了就当文本。编排 Agent（Yosuri）用
	// rp_set_stat 工具写它，用户也能在角色面板里手填。
	Custom []CustomStat `json:"custom,omitempty"`
}

// InvItem 背包里的一格物品。
type InvItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Icon  string `json:"icon,omitempty"` // 展示用 emoji（可空）
	Desc  string `json:"desc,omitempty"` // 一句话说明
}

// CustomStat 一条自定义人物参数。
type CustomStat struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HasStats 是否设定过任何人物参数（全零 = 未设定）。
func (s AgentStats) HasStats() bool {
	return s.Level != 0 || s.MaxHP != 0 || s.HP != 0 || s.MaxMP != 0 || s.MP != 0 ||
		s.Shield != 0 || s.ATK != 0 || s.DEF != 0 || s.SPD != 0 || s.MAG != 0 ||
		s.Gold != 0 || s.Affection != 0 || s.Element != "" ||
		len(s.Equipment) > 0 || s.Title != "" || len(s.Marks) > 0 || len(s.Inventory) > 0 || len(s.Custom) > 0
}

// ExpForLevel / LevelUp 经验与升级规则（RP 战斗胜利发经验，累积升级）。
// 阈值 = level*100 经验升 1 级，封顶 Lv.99。升级按职业系数涨属性：
// 战士攻防大、法师魔大、其余均衡——没有职业就用均衡系数。
const maxAgentLevel = 99

func expThreshold(level int) int {
	if level <= 0 {
		return 100
	}
	return level * 100
}

// levelUpIfNeeded 结算后调用：经验够就连续升级（可一次跳多级），并回满血魔。
func levelUpIfNeeded(st AgentStats) AgentStats {
	for st.Level < maxAgentLevel && st.Exp >= expThreshold(st.Level) {
		st.Exp -= expThreshold(st.Level)
		st.Level++

		// 成长系数：有面具职业按职业，否则均衡。
		// 血魔按百分比成长（MaxHP 先存在才有得涨），攻防速魔保底 +2。
		hpGrow := 12
		if st.MaxHP > 0 {
			hpGrow = st.MaxHP / 8
		}
		mpGrow := 8
		if st.MaxMP > 0 {
			mpGrow = st.MaxMP / 8
		}
		switch st.Element {
		case "fire", "earth", "light":
			st.ATK += 3
			st.DEF += 3
			st.SPD += 2
			st.MAG += 1
		case "water", "dark", "holy":
			st.MAG += 3
			st.ATK += 2
			st.DEF += 2
			st.SPD += 1
		default: // 均衡
			st.ATK += 2
			st.DEF += 2
			st.SPD += 2
			st.MAG += 2
		}
		if hpGrow > 0 {
			st.MaxHP += hpGrow
			st.HP += hpGrow
		}
		if mpGrow > 0 {
			st.MaxMP += mpGrow
			st.MP += mpGrow
		}
	}
	return st
}

// rpStatSummary 给编排 Agent/前端的中文状态摘要（含等级/经验进度）。
func (s AgentStats) rpStatSummary() string {
	if s.Level == 0 && s.MaxHP == 0 {
		return ""
	}
	need := expThreshold(s.Level)
	return fmt.Sprintf("Lv.%d 经验%d/%d HP%d/%d MP%d/%d 攻%d 防%d 速%d 魔%d 金币%d 好感%d",
		s.Level, s.Exp, need, s.HP, s.MaxHP, s.MP, s.MaxMP,
		s.ATK, s.DEF, s.SPD, s.MAG, s.Gold, s.Affection)
}

// AgentCard 一个 Agent 的角色卡（不含头像二进制，头像单独落文件）。
type AgentCard struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Persona   string     `json:"persona"`
	Avatar    string     `json:"avatar,omitempty"` // base64 dataURL，读接口时现拼
	Icon      string     `json:"icon,omitempty"`   // 无头像时的图标兜底（mdi:xxx）
	Color     string     `json:"color,omitempty"`  // 头像底色/名牌色
	Character string     `json:"character,omitempty"`
	Stats     AgentStats `json:"stats,omitempty"` // 人物参数（等级/血量/装备/好感）
	Portrait  string     `json:"portrait,omitempty"`
	CreatedAt string     `json:"created_at,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
}

var agentRegistry = struct {
	sync.Mutex
	cached  []AgentCard
	cachedM time.Time
}{}

// agentsFilePath 注册表落盘路径（跟随 RESCENE_DATA_DIR，与头像/记忆同域）。
func agentsFilePath() string {
	return filepath.Join(resceneUserDataDir(), "agents.json")
}

// readAgentCards 读注册表（带 2s 内存缓存，避免每轮工作流都解一遍 JSON）。
func readAgentCards() []AgentCard {
	agentRegistry.Lock()
	defer agentRegistry.Unlock()
	if time.Since(agentRegistry.cachedM) < 2*time.Second && agentRegistry.cached != nil {
		out := make([]AgentCard, len(agentRegistry.cached))
		copy(out, agentRegistry.cached)
		return out
	}
	data, err := os.ReadFile(agentsFilePath())
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		agentRegistry.cached = []AgentCard{}
		agentRegistry.cachedM = time.Now()
		return nil
	}
	var cards []AgentCard
	if err := json.Unmarshal(data, &cards); err != nil {
		return nil
	}
	agentRegistry.cached = cards
	agentRegistry.cachedM = time.Now()
	out := make([]AgentCard, len(cards))
	copy(out, cards)
	return out
}

// saveAgentCards 原子写注册表并刷新缓存。
func saveAgentCards(cards []AgentCard) error {
	data, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return err
	}
	path := agentsFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	agentRegistry.Lock()
	agentRegistry.cached = cards
	agentRegistry.cachedM = time.Now()
	agentRegistry.Unlock()
	return nil
}

// GetAgentCard 按 id 取角色卡（id 为空时返回 nil）。
func GetAgentCard(id string) *AgentCard {
	id = memorydir.SanitizeAgentID(id)
	if id == "" {
		return nil
	}
	for _, c := range readAgentCards() {
		if c.ID == id {
			cc := c
			return &cc
		}
	}
	return nil
}

// agentCardAvatar 读某个 agent 的头像文件（base64 dataURL；不存在返回空）。
func agentCardAvatar(id string) string {
	id = memorydir.SanitizeAgentID(id)
	if id == "" {
		return ""
	}
	data, err := os.ReadFile(memorydir.AgentAvatarPath(id))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// fillAgentAvatar 把头像填进卡片（注册表本体不含头像，读接口现拼）。
func fillAgentAvatar(c *AgentCard) {
	if c == nil {
		return
	}
	if av := agentCardAvatar(c.ID); av != "" {
		c.Avatar = av
	}
}

// ListAgents 返回全部角色卡（带头像），用于前端设置页与群聊装配。
func ListAgents() []AgentCard {
	cards := readAgentCards()
	out := make([]AgentCard, 0, len(cards))
	for _, c := range cards {
		cc := c
		fillAgentAvatar(&cc)
		out = append(out, cc)
	}
	return out
}

// AgentPersona 取某个 agent 的人设文案（不存在返回空）。
func AgentPersona(id string) string {
	c := GetAgentCard(id)
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Persona)
}

// maxAgentCards 单用户角色卡上限：群聊一轮最多点名几个，太多没意义。
const maxAgentCards = 24

// slugifyAgentName 从名字生成 id 候选（保留字母数字，其余转连字符）。
// 中文名字母全被剥掉时回退时间戳 id，保证 id 唯一且合法。
func slugifyAgentName(name string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) < 2 {
		return ""
	}
	if len(s) > 24 {
		s = s[:24]
	}
	return memorydir.SanitizeAgentID(s)
}

// newAgentID 生成合法且不与现有卡片冲突的 id。
func newAgentID(name string, cards []AgentCard) string {
	base := slugifyAgentName(name)
	if base == "" {
		base = "agent"
	}
	exists := map[string]bool{}
	for _, c := range cards {
		exists[c.ID] = true
	}
	if !exists[base] {
		return base
	}
	for i := 2; i < 1000; i++ {
		cand := memorydir.SanitizeAgentID(fmt.Sprintf("%s-%d", base, i))
		if cand != "" && !exists[cand] {
			return cand
		}
	}
	return ""
}

// UpsertAgent 新建或更新一张角色卡（id 为空则按名字生成）。
// 头像单独走 SaveAgentAvatar，这里只处理元信息。
func UpsertAgent(card AgentCard) (AgentCard, error) {
	cards := readAgentCards()
	card.Name = strings.TrimSpace(card.Name)
	if card.Name == "" {
		return card, fmt.Errorf("名字不能为空")
	}
	if len([]rune(card.Name)) > 32 {
		card.Name = string([]rune(card.Name)[:32])
	}
	if len([]rune(card.Persona)) > 8000 {
		return card, fmt.Errorf("角色卡文案过长（上限 8000 字）")
	}
	now := time.Now().Format("2006-01-02 15:04")

	id := memorydir.SanitizeAgentID(card.ID)
	if id == "" {
		id = newAgentID(card.Name, cards)
		if id == "" {
			return card, fmt.Errorf("角色卡数量已达上限")
		}
		card.ID = id
		card.CreatedAt = now
	}
	card.UpdatedAt = now
	card.Avatar = "" // 头像不进注册表

	for i := range cards {
		if cards[i].ID == id {
			if cards[i].CreatedAt != "" {
				card.CreatedAt = cards[i].CreatedAt
			}
			cards[i] = card
			return card, saveAgentCards(cards)
		}
	}
	if len(cards) >= maxAgentCards {
		return card, fmt.Errorf("最多创建 %d 个 Agent", maxAgentCards)
	}
	cards = append(cards, card)
	return card, saveAgentCards(cards)
}

// DeleteAgent 删除一张角色卡，连带它的私有记忆目录与头像。
// 只删 rescene_data/agents/<id>/ 下的东西，通用记忆一律不动。
func DeleteAgent(id string) error {
	id = memorydir.SanitizeAgentID(id)
	if id == "" {
		return fmt.Errorf("agent id 非法")
	}
	cards := readAgentCards()
	out := cards[:0]
	found := false
	for _, c := range cards {
		if c.ID == id {
			found = true
			continue
		}
		out = append(out, c)
	}
	if !found {
		return nil // 幂等：本来就没有
	}
	if err := saveAgentCards(out); err != nil {
		return err
	}
	return os.RemoveAll(memorydir.AgentDir(id))
}

// SaveAgentAvatar 落盘某个 agent 的头像（base64 dataURL）。
// 传空串等于清除头像（回退到 icon/名字首字）。
func SaveAgentAvatar(id, dataURL string) error {
	id = memorydir.SanitizeAgentID(id)
	if id == "" {
		return fmt.Errorf("agent id 非法")
	}
	if GetAgentCard(id) == nil {
		return fmt.Errorf("Agent 不存在")
	}
	p := memorydir.AgentAvatarPath(id)
	if strings.TrimSpace(dataURL) == "" {
		return os.Remove(p)
	}
	if len(dataURL) > 3*1024*1024 {
		return fmt.Errorf("头像过大（上限 3MB）")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(dataURL), 0o644)
}
