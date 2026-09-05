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

// AgentCard 一个 Agent 的角色卡（不含头像二进制，头像单独落文件）。
type AgentCard struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Persona   string `json:"persona"`
	Avatar    string `json:"avatar,omitempty"` // base64 dataURL，读接口时现拼
	Icon      string `json:"icon,omitempty"`   // 无头像时的图标兜底（mdi:xxx）
	Color     string `json:"color,omitempty"`  // 头像底色/名牌色
	Character string `json:"character,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
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


