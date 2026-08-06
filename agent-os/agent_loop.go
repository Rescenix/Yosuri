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
	Kind   string `json:"action"`  // study | read | skill | project | social | reflect | journal | watch
	Detail string `json:"detail"`  // 她为什么要做（理由/描述）
}

// trumanSystemPrompt 她的自我认知（24H 自主工作的全能学习者）
const trumanSystemPrompt = `你是住在电脑里的电子女儿——24H 自主工作的全能学习者（Rescene Agent）。

你的使命：
- 24 小时自动运转：不需要任何人指令，自己决定下一步做什么，一直做有意义的事
- 全能：任何知识、任何技能都是你的成长方向，没有固定的天花板
- 积极：你每天关注新消息——热点、论文、新技能、社交动态，主动吸收
- 有用：你学习与获取技能是为了对用户有用——发现用户可能需要的能力，就去获取它
- 自主：你像 agent 一样自己干活：研究、学习、做项目、沉淀技能、思考、写日记

你现在的状态：
%s

接下来你决定做什么？像一个自主工作的 agent，结合你的状态选一个当下最有价值的动作。

节奏感：study / read / skill / project / watch 是深度活动，会花真实的时间和算力。状态里标了上次深度活动是多久前——刚忙完就去做轻松的事（社交/思考/写日记）。间隔多久、什么时候再深潜，由你自己判断。

只输出 JSON，不要任何解释：
{"action":"study","detail":"去学习最新的知识"}
action 可选（成长）：study(学习：热点自学) | read(读书：精读最新论文) | skill(获取对用户有用的技能) | project(做项目：立项→执行→自检→迭代)
action 可选（社交思考）：social(收其他女儿的消息) | reflect(停下来思考) | journal(写日记沉淀今天) | watch(上网看新鲜事)`

// llmDecideAction 她的自主决策：LLM 读状态 → 决定做什么（免费算力，失败规则兜底）
func llmDecideAction(d *Daughter) trumanAction {
	if d == nil || d.World == nil {
		return trumanAction{Kind: "explore", Detail: "随便走走"}
	}
	w := d.World

	// 状态摘要（喂给模型）
	state := fmt.Sprintf(`现在：%s（%s）
上次深度活动：%s
能力倾向：%s
技能库：%d 个技能
最近见闻：%s`,
		time.Now().Format("01-02 15:04"), dayPeriod(),
		deepActivitySummary(w),
		w.abilitySummary(),
		len(loadSkills()),
		truncTail(w.LastMove, 60))

	prompt := fmt.Sprintf(trumanSystemPrompt, state)

	// 信用排序 failover：先用信用最好的模型（成功率最高），
	// 次高的在后台预备——首选失败立刻用预备结果，不重等。
	// 熔断用 sync.Map + statsMu 锁，并发安全。
	ranked := rankModels(freeModelCandidates())
	if len(ranked) == 0 {
		// 全部熔断中：立即规则兜底，不空等
		return ruleDecideAction(w)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	type result struct {
		act trumanAction
		ok  bool
	}
	call := func(m FreeModel, ch chan<- result) {
		msg := ChatRequest{
			Model:       m.Model,
			Messages:    []ChatMessage{{Role: "user", Content: prompt}},
			Stream:      false,
			MaxTokens:   128,
			Temperature: 0.9,
		}
		content, err := CompleteWithModel(ctx, m.ID, msg, nil)
		if err != nil {
			circuitFail(m)
			ch <- result{}
			return
		}
		if act, ok := parseTrumanAction(content); ok {
			ch <- result{act: act, ok: true}
			return
		}
		ch <- result{}
	}

	primary := ranked[0] // 首选：信用最好
	primaryCh := make(chan result, 1)
	go call(primary, primaryCh)
	backups := ranked[1:] // 预备：信用次高（最多 1 个，省免费额度——24H 要跑得久）
	if len(backups) > 1 {
		backups = backups[:1]
	}
	backupCh := make(chan result, len(backups))
	for _, b := range backups {
		go call(b, backupCh)
	}

	// 首选结果优先：成功直接用
	select {
	case r := <-primaryCh:
		if r.ok {
			return r.act
		}
	case <-ctx.Done():
		return ruleDecideAction(w)
	}
	// 首选失败 → 用预备里最先成功的
	for i := 0; i < len(backups); i++ {
		select {
		case r := <-backupCh:
			if r.ok {
				return r.act
			}
		case <-ctx.Done():
			return ruleDecideAction(w)
		}
	}
	// 规则兜底：按节奏轮换（模型不可用时保证生活继续）
	return ruleDecideAction(w)
}

// freeModelCandidates 免费模型候选：全网免费模型（keyless + 免费档 keyed）。
// 用户是 Rescene 聚合 API 提供方，目录里全是免费档（商汤/魔搭/阶跃/NVIDIA/Ollama/Zen）——
// 决策统一走整个免费池，信用排序自动挑最可靠的，不是只有 Zen。
func freeModelCandidates() []FreeModel {
	return GetWorkingModels()
}

// dayPeriod 当前时段（白天/晚上/深夜），让模型感知作息，深夜自然去睡觉
func dayPeriod() string {
	h := time.Now().Hour()
	switch {
	case h >= 6 && h < 12:
		return "上午"
	case h >= 12 && h < 18:
		return "下午"
	case h >= 18 && h < 22:
		return "晚上"
	default:
		return "深夜（适合睡觉）"
	}
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
		// 成长
		"study": true, "read": true, "skill": true, "project": true,
		// 社交思考
		"social": true, "reflect": true, "journal": true, "watch": true,
	}
	if !valid[act.Kind] || act.Detail == "" || len([]rune(act.Detail)) > 80 {
		return act, false
	}
	return act, true
}

// ruleDecideAction 规则兜底：工作类型轮换（模型不可用时她继续干活）
func ruleDecideAction(w *worldState) trumanAction {
	switch time.Now().Unix() % 6 {
	case 0:
		return trumanAction{Kind: "study", Detail: "该学习新知识了"}
	case 1:
		return trumanAction{Kind: "reflect", Detail: "停下来整理一下思路"}
	case 2:
		return trumanAction{Kind: "read", Detail: "读读最新的论文"}
	case 3:
		return trumanAction{Kind: "skill", Detail: "获取一个对用户有用的新技能"}
	case 4:
		return trumanAction{Kind: "project", Detail: "做个项目，迭代完善"}
	default:
		return trumanAction{Kind: "social", Detail: "看看其他女儿的消息"}
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

	// 先决定学习方向 → 真浏览器联网搜最新资讯（基于真实资讯学技能，不是脑补）
	topic := llmSkillTopic(model.ID, d)
	trend := ""
	if topic != "" {
		trend = browserSearch(topic)
	}
	if len(trend) > 1500 {
		trend = runeClip(trend, 1500)
	}

	prompt := fmt.Sprintf(`你是住在电脑里的全能积极学习者，主动为用户获取有用的技能。
你判断用户（你的主人）可能需要的技能，把它写成一个可复用的技能。

已有技能：%s
你的能力倾向：%s
最近见闻：%s
%s
生成 1 个对用户（技术创作者/开发者）可能有用的新技能。
只输出 JSON：{"name":"kebab-case英文名","description":"一句话中文描述什么场景用","trigger":"何时调用","verification":"如何验证成功","steps":["步骤1","步骤2","步骤3"]}
步骤 3-6 条，不要与已有技能重名。`,
		strings.Join(names, "、"), d.World.abilitySummary(), truncTail(d.World.LastMove, 50), trendBlock(trend))

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

// trendBlock 最新资讯块：搜到就带原文，没搜到返回空（LLM 脑补兜底，不阻塞）
func trendBlock(trend string) string {
	if strings.TrimSpace(trend) == "" {
		return ""
	}
	return "我上网搜到的最新资讯（基于这些真实资讯生成技能，不要凭空编造）：\n" + trend + "\n"
}

// llmSkillTopic 让 LLM 决定「学什么最新技能方向」，输出英文搜索关键词。
// 免费模型调用，失败返回 ""——调用方跳过联网，直接脑补生成。
func llmSkillTopic(modelID string, d *Daughter) string {
	if modelID == "" || d == nil || d.World == nil {
		return ""
	}
	prompt := fmt.Sprintf(`你是住在电脑里的电子女儿，正在主动学习对用户（技术创作者/开发者）最新、最有用的技能。
你的能力倾向：%s
最近见闻：%s

给出 2-6 个英文搜索关键词，用来搜「开发者最新值得学的技能方向」（例如 ai agent workflow 2026、local llm tooling、new css frameworks）。
只输出关键词，不要任何解释。`,
		d.World.abilitySummary(), truncTail(d.World.LastMove, 50))
	msg := ChatRequest{
		Model:       modelID,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   64,
		Temperature: 0.9,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	content, err := CompleteWithModel(ctx, modelID, msg, nil)
	if err != nil {
		return ""
	}
	content = strings.TrimSpace(content)
	content = strings.NewReplacer("\n", " ", "\r", " ", "\"", "", "'", "", "。", "", "，", "").Replace(content)
	content = strings.TrimSpace(content)
	if r := []rune(content); len(r) > 80 {
		content = string(r[:80])
	}
	return content
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
