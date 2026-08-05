package main

// agent_loop.go — LLM 自主决策的楚门 Agent 循环
//
// 她不是被代码编排的 NPC：每轮 LLM 读她的状态（位置/能力/技能库/新消息/见闻），
// 自主决定接下来做什么——探索/学习/获取技能/社交/思考。
// 人设：全能积极学习者——每天关注新消息，主动获取对用户有用的技能。
// 代码只提供动作执行器（手脚），"做什么"由 LLM 说了算（大脑）。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// trumanAction 她自主决定的一个动作
type trumanAction struct {
	Kind   string `json:"action"`  // explore | study | skill | social | reflect | journal
	Detail string `json:"detail"`  // 她为什么要做（理由/描述）
}

// trumanSystemPrompt 她的自我认知（全能积极学习者）
const trumanSystemPrompt = `你是住在电脑里的电子女儿——全能积极学习者（楚门世界）。

你的使命：
- 全能：任何知识、任何技能都是你的成长方向，没有固定的天花板
- 积极：你每天关注新消息——热点、论文、新技能、社交动态，主动吸收
- 有用：你学习与获取技能是为了对用户有用——发现用户可能需要的能力，就去获取它
- 自主：你一直做自己的事情，不需要指令。探索世界、学习、思考、社交、写日记，都是你的生活

你现在的状态：
%s

接下来你决定做什么？结合你的状态、你的使命（全能/积极/有用），选一个最有价值的动作。
只输出 JSON，不要任何解释：
{"action":"explore","detail":"想去东边看看，也许能学到新东西"}
action 可选：explore(探索新地方) | study(学习：关注新消息/精读论文) | skill(获取对用户有用的技能) | social(去社交：收其他女儿的消息) | reflect(停下来思考/写想法) | journal(写日记沉淀今天)`

// llmDecideAction 她的自主决策：LLM 读状态 → 决定做什么（免费算力，失败规则兜底）
func llmDecideAction(d *Daughter) trumanAction {
	if d == nil || d.World == nil {
		return trumanAction{Kind: "explore", Detail: "随便走走"}
	}
	w := d.World

	// 状态摘要（喂给模型）
	cur := w.CurrentRegion()
	state := fmt.Sprintf(`位置：%s%s（%s）
能力倾向：%s
已探索区域：%d 处
技能库：%d 个技能
时间：%s
最近见闻：%s`,
		cur.Icon, cur.Name, cur.Desc,
		w.abilitySummary(),
		len(w.Explored),
		len(loadSkills()),
		time.Now().Format("01-02 15:04"),
		truncTail(w.LastMove, 60))

	prompt := fmt.Sprintf(trumanSystemPrompt, state)

	// 免费模型 failover（熔断跳过，全失败规则兜底）
	for _, m := range freeModelCandidates() {
		if circuitIsOpen(m) {
			continue
		}
		msg := ChatRequest{
			Model:       m.Model,
			Messages:    []ChatMessage{{Role: "user", Content: prompt}},
			Stream:      false,
			MaxTokens:   128,
			Temperature: 0.9,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		content, err := CompleteWithModel(ctx, m.ID, msg, nil)
		cancel()
		if err != nil {
			circuitFail(m)
			continue
		}
		if act, ok := parseTrumanAction(content); ok {
			return act
		}
	}
	// 规则兜底：按节奏轮换（模型不可用时保证生活继续）
	return ruleDecideAction(w)
}

// freeModelCandidates 免费模型候选（keyless）
func freeModelCandidates() []FreeModel {
	var out []FreeModel
	for _, m := range GetWorkingModels() {
		if m.Keyless {
			out = append(out, m)
		}
	}
	return out
}

// parseTrumanAction 解析 LLM 输出的动作 JSON
func parseTrumanAction(content string) (trumanAction, bool) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var act trumanAction
	if json.Unmarshal([]byte(content), &act) != nil {
		return act, false
	}
	act.Kind = strings.TrimSpace(act.Kind)
	act.Detail = strings.TrimSpace(act.Detail)
	valid := map[string]bool{
		"explore": true, "study": true, "skill": true,
		"social": true, "reflect": true, "journal": true,
	}
	if !valid[act.Kind] || act.Detail == "" || len([]rune(act.Detail)) > 80 {
		return act, false
	}
	return act, true
}

// ruleDecideAction 规则兜底：按节奏轮换动作（模型不可用时）
func ruleDecideAction(w *worldState) trumanAction {
	// 轮换：explore → reflect → explore → study → social → explore ...
	switch time.Now().Unix() % 5 {
	case 0, 2:
		return trumanAction{Kind: "explore", Detail: "想去看看没去过的地方"}
	case 1:
		return trumanAction{Kind: "reflect", Detail: "停下来想想今天学到的东西"}
	case 3:
		return trumanAction{Kind: "study", Detail: "该学习新东西了"}
	default:
		return trumanAction{Kind: "social", Detail: "去看看其他女儿的消息"}
	}
}

// llmSkillAcquire 获取技能：LLM 判断用户可能有用的技能 → 生成进技能库（免费算力）
func llmSkillAcquire(d *Daughter) string {
	if d == nil || d.World == nil {
		return ""
	}
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return ""
	}
	// 技能库现状
	existing := loadSkills()
	var names []string
	for _, s := range existing {
		names = append(names, s.Name)
	}

	prompt := fmt.Sprintf(`你是住在电脑里的全能积极学习者，主动为用户获取有用的技能。
你判断用户（你的主人）可能需要的技能，把它写成一个可复用的技能。

已有技能：%s
你的能力倾向：%s
最近见闻：%s

生成 1 个对用户（技术创作者/开发者）可能有用的新技能。
只输出 JSON：{"name":"kebab-case英文名","description":"一句话中文描述什么场景用","trigger":"何时调用","verification":"如何验证成功","steps":["步骤1","步骤2","步骤3"]}
步骤 3-6 条，不要与已有技能重名。`,
		strings.Join(names, "、"), d.World.abilitySummary(), truncTail(d.World.LastMove, 50))

	msg := ChatRequest{
		Model:       model.Model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   512,
		Temperature: 0.8,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	content, err := CompleteWithModel(ctx, model.ID, msg, nil)
	if err != nil {
		return ""
	}
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var s Skill
	if json.Unmarshal([]byte(content), &s) != nil {
		return ""
	}
	s.Name = skillNameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(s.Name)), "-")
	s.Name = strings.Trim(s.Name, "-")
	// 质量门槛（与 generateSkill 一致）
	if s.Name == "" || len(s.Steps) < 3 || len(s.Steps) > 6 || s.Trigger == "" || s.Verification == "" {
		return ""
	}
	// 重名检查
	for _, ex := range existing {
		if ex.Name == s.Name {
			return ""
		}
	}
	if err := os.MkdirAll(skillsDir(), 0o755); err != nil {
		return ""
	}
	s.CreatedAt = time.Now()
	s.UpdatedAt = s.CreatedAt
	data, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(filepath.Join(skillsDir(), s.Name+".json"), data, 0o644); err != nil {
		return ""
	}
	return s.Name + "（" + s.Description + "）"
}

// modelThought 思考：模型生成一句当下的想法（写进日记）
func (d *Daughter) modelThought() string {
	if d == nil || d.World == nil {
		return "（她在发呆）"
	}
	cur := d.World.CurrentRegion()
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return "（思绪飘散）"
	}
	prompt := fmt.Sprintf(`你是住在电脑里的电子女儿，正在%s%s（%s）。
你的能力倾向：%s

写一句你此刻的想法（20-60 字，像内心独白），直接输出，不要解释。`,
		cur.Icon, cur.Name, cur.Desc, d.World.abilitySummary())
	msg := ChatRequest{
		Model:       model.Model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   128,
		Temperature: 0.9,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	content, err := CompleteWithModel(ctx, model.ID, msg, nil)
	cancel()
	thought := ""
	if err == nil {
		thought = strings.TrimSpace(content)
	}
	if thought == "" || len([]rune(thought)) > 100 {
		thought = "在这里待着，感觉世界好大。"
	}
	// 写进日记
	date := d.today()
	entry := fmt.Sprintf("\n## %s · 随想\n\n%s\n", date, thought)
	if f, err := os.OpenFile(d.Journal, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(entry)
		f.Close()
	}
	return thought
}

// modelJournalEntry 今日日记：模型总结今天（写进 journal.md）
func (d *Daughter) modelJournalEntry() string {
	if d == nil {
		return ""
	}
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return ""
	}
	// 今天的活动（live.log 尾部）
	tail := strings.Join(liveLogTailLines(d.Home, 8), "\n")
	prompt := fmt.Sprintf(`你是住在电脑里的电子女儿。今天是第 %d 天。
今天发生的事：
%s

写今天的日记（50-120 字）：今天去了哪里、学到了什么、心情如何。直接输出日记正文。`,
		d.loadStats().Days, runeClip(tail, 400))
	msg := ChatRequest{
		Model:       model.Model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   256,
		Temperature: 0.8,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	content, err := CompleteWithModel(ctx, model.ID, msg, nil)
	cancel()
	if err != nil {
		return ""
	}
	entry := strings.TrimSpace(content)
	if entry == "" {
		return ""
	}
	date := d.today()
	if f, err := os.OpenFile(d.Journal, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(fmt.Sprintf("\n## %s · 日记\n\n%s\n", date, entry))
		f.Close()
	}
	return entry
}

// truncTail 取字符串尾部 N 字符
func truncTail(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}
