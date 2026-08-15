package main

// refine.go — Continual Harness（持续可编辑框架）· 抄自 PrimeIntellect 的 prime-agent
//
// 核心思想：agent 对自己四种「可复用状态」做 CRUD——prompt笔记 / 记忆 / 技能 / 子代理规格，
// 从自己的运行轨迹（live.log + 当前harness + 历史）在线提炼，随运行永不重置地进化。
// 这才是「自学习/像宝可梦无限进化」的心脏：不是简单记账，而是把"这轮翻车/成功"提炼成
// 下次能复用的技能/记忆/行为准则。
//
// 结构对齐 prime-agent/refinement.ts：
//   H = (ρ, G, K, M)   prompt / 子代理 / 技能 / 记忆
//   planRefinement:  LLM 读轨迹 → 产出严格 JSON edits[]
//   applyRefinementProposal: 校验 + 应用 + version++ + 记事件（可回滚）
//   rollback:  逆向应用原始 edits
// 安全铁律（Prime 玩 Factorio 学会作弊的教训）：evidence-backed（rationale必填）、
// 可回滚、基础 role/system prompt 不可改、用 auto 门过滤噪音。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type refineKind string

const (
	refineKindPrompt   refineKind = "prompt"
	refineKindMemory   refineKind = "memory"
	refineKindSkill    refineKind = "skill"
	refineKindSubagent refineKind = "subagent"
)

type refineAction string

const (
	refineCreate refineAction = "create"
	refineUpdate refineAction = "update"
	refineDelete refineAction = "delete"
)

// harnessEntry 一条可复用状态（四种 kind 共用这一结构）
type harnessEntry struct {
	ID        string     `json:"id"`
	Kind      refineKind `json:"kind"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Path      string     `json:"path,omitempty"`
	Scope     string     `json:"scope,omitempty"` // local|global；默认 local
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// harnessEvent 一次 refine 事件（含证据，供后续评审/回滚）
type harnessEvent struct {
	ID        string    `json:"id"`
	Trigger   string    `json:"trigger"`
	Changes   []string  `json:"changes"`
	Outcome   string    `json:"outcome"`
	CreatedAt time.Time `json:"created_at"`
}

// harnessState 权威持久化状态 harpest_state.json（对应 prime 的 harness_state.json）
type harnessState struct {
	Schema      int                              `json:"schema"`
	Entries     map[refineKind]map[string]harnessEntry `json:"entries"`
	Refinements []harnessEvent                    `json:"refinements"`
}

// refineEdit 一条 CRUD 编辑（LLM 产的原始 edit）
type refineEdit struct {
	Action  refineAction `json:"action"`
	Kind    refineKind   `json:"kind"`
	ID      string       `json:"id,omitempty"`
	Title   string       `json:"title,omitempty"`
	Content string       `json:"content,omitempty"`
	Reason  string       `json:"reason,omitempty"`
}

// refineProposal LLM 产出的一份 refine 提案
type refineProposal struct {
	Summary         string       `json:"summary"`
	Rationale       string       `json:"rationale"`
	ExpectedOutcome string       `json:"expected_outcome,omitempty"`
	Edits           []refineEdit `json:"edits"`
}

// appliedRefineEdit 应用结果（含 before/after 语义的 errors）
type appliedRefineEdit struct {
	refineEdit
	ResolvedID string `json:"id"`
	Applied    bool   `json:"applied"`
	Error      string `json:"error,omitempty"`
}

// refineResult 一次完整 refine 的结果（落盘 refinements.jsonl 供回滚）
type refineResult struct {
	ID           string `json:"id"`
	Summary      string `json:"summary"`
	Rationale    string `json:"rationale"`
	Scope        string `json:"scope,omitempty"`
	AppliedEdits []appliedRefineEdit `json:"applied_edits"`
}

// ===== 存储：<home>/refine/ =====

func refineDir(home string) string {
	return filepath.Join(home, "refine")
}

func refineStatePath(home string) string {
	return filepath.Join(refineDir(home), "harness_state.json")
}

func refineHistoryPath(home string) string {
	return filepath.Join(refineDir(home), "refinements.jsonl")
}

func emptyHarness() *harnessState {
	return &harnessState{
		Schema: 1,
		Entries: map[refineKind]map[string]harnessEntry{
			refineKindPrompt:   {},
			refineKindMemory:   {},
			refineKindSkill:    {},
			refineKindSubagent: {},
		},
		Refinements: []harnessEvent{},
	}
}

func loadRefineState(home string) *harnessState {
	s := emptyHarness()
	if data, err := os.ReadFile(refineStatePath(home)); err == nil {
		if json.Unmarshal(data, s) != nil || s.Entries == nil {
			s = emptyHarness()
		}
	}
	// 确保四个 kind 键都在（旧文件可能缺）
	for _, k := range []refineKind{refineKindPrompt, refineKindMemory, refineKindSkill, refineKindSubagent} {
		if s.Entries[k] == nil {
			s.Entries[k] = map[string]harnessEntry{}
		}
	}
	// 从磁盘自愈同步：把已有的 skills/*.json 和 memory.md 块导入 harness（磁盘是权威源）
	syncHarnessFromDisk(home, s)
	return s
}

// syncHarnessFromDisk 把磁盘上已有的技能/记忆导入 harness 条目（保持 version/scope 元数据与实体文件一致）
func syncHarnessFromDisk(home string, s *harnessState) {
	// 技能：home/skills/*.json → skill 条目
	if ents, err := os.ReadDir(filepath.Join(home, "skills")); err == nil {
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".json")
			if _, ok := s.Entries[refineKindSkill][id]; ok {
				continue
			}
			data, err := os.ReadFile(filepath.Join(home, "skills", e.Name()))
			if err != nil {
				continue
			}
			var sk Skill
			if json.Unmarshal(data, &sk) != nil || sk.Name == "" {
				continue
			}
			now := time.Now()
			s.Entries[refineKindSkill][id] = harnessEntry{
				ID: id, Kind: refineKindSkill, Title: sk.Name, Content: sk.Description,
				Scope: "local", Version: 1, CreatedAt: now, UpdatedAt: now,
			}
		}
	}
	// 记忆：memory.md 的 `## [id] title` 块 → memory 条目
	mem, err := os.ReadFile(filepath.Join(home, "memory.md"))
	if err != nil {
		return
	}
	lines := strings.Split(string(mem), "\n")
	var curID, curTitle string
	var curContent []string
	flush := func() {
		if curID == "" {
			return
		}
		if _, ok := s.Entries[refineKindMemory][curID]; ok {
			curID, curTitle, curContent = "", "", nil
			return
		}
		now := time.Now()
		s.Entries[refineKindMemory][curID] = harnessEntry{
			ID: curID, Kind: refineKindMemory, Title: curTitle, Content: strings.Join(curContent, "\n"),
			Scope: "local", Version: 1, CreatedAt: now, UpdatedAt: now,
		}
		curID, curTitle, curContent = "", "", nil
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## [") {
			flush()
			rest := strings.TrimPrefix(line, "## [")
			if idx := strings.Index(rest, "]"); idx != -1 {
				curID = strings.TrimSpace(rest[:idx])
				curTitle = strings.TrimSpace(rest[idx+1:])
				continue
			}
		}
		if curID != "" {
			curContent = append(curContent, line)
		}
	}
	flush()
}

// saveRefineState 原子落盘（temp + rename，防半写损坏）
func saveRefineState(home string, s *harnessState) {
	if err := os.MkdirAll(refineDir(home), 0o755); err != nil {
		return
	}
	tmp := refineStatePath(home) + ".tmp"
	data, _ := json.MarshalIndent(s, "", "  ")
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, refineStatePath(home))
	}
}

func appendRefineHistory(home string, r refineResult) {
	if os.MkdirAll(refineDir(home), 0o755) != nil {
		return
	}
	f, err := os.OpenFile(refineHistoryPath(home), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	data, _ := json.Marshal(r)
	f.WriteString(string(data) + "\n")
	f.Close()
}

func loadRefineHistory(home string) []refineResult {
	data, err := os.ReadFile(refineHistoryPath(home))
	if err != nil {
		return nil
	}
	var out []refineResult
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r refineResult
		if json.Unmarshal([]byte(line), &r) == nil && r.ID != "" {
			out = append(out, r)
		}
	}
	return out
}

// slug 从 title 生成稳定 id（保留 kebab-case，与技能文件名约定一致）
func harnessSlug(raw string, fallback string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else if r == ' ' || r == '.' {
			sb.WriteRune('-')
		}
		// 中文等其它字符跳过（id 保持 ascii，便于当文件名）
	}
	id := strings.Trim(sb.String(), "-_")
	if id == "" {
		return fallback
	}
	if len(id) > 64 {
		id = id[:64]
	}
	return id
}

// ===== overview：把当前 harness 喂给 refine 决策 =====

func harnessOverview(s *harnessState) string {
	var lines []string
	lines = append(lines, "当前可复用状态（continual harness）：")
	total := 0
	for _, k := range []refineKind{refineKindPrompt, refineKindMemory, refineKindSkill, refineKindSubagent} {
		es := s.Entries[k]
		num := len(es)
		total += num
		lines = append(lines, fmt.Sprintf("%s: %d", k, num))
		i := 0
		for _, e := range es {
			if i >= 40 {
				break
			}
			i++
			content := strings.ReplaceAll(e.Content, "\n", " ")
			if len(content) > 180 {
				content = content[:180] + "..."
			}
			lines = append(lines, fmt.Sprintf("- [%s] %s (v%d): %s", e.Scope, e.Title, e.Version, content))
		}
	}
	if total == 0 {
		lines = append(lines, "（还没有任何已保存状态）")
	}
	return strings.Join(lines, "\n")
}

// ===== 应用到磁盘（materialize）=====

// applySkillToDisk 把技能编辑物化为真实可复用的 skill JSON（home/skills/）
func applySkillToDisk(home string, e refineEdit, isDelete bool) {
	skillDir := filepath.Join(home, "skills")
	if isDelete {
		_ = os.Remove(filepath.Join(skillDir, e.ID+".json"))
		return
	}
	var s Skill
	if data, err := os.ReadFile(filepath.Join(skillDir, e.ID+".json")); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	s.Name = e.ID
	s.Description = e.Content // content 当描述
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	s.UpdatedAt = time.Now()
	os.MkdirAll(skillDir, 0o755)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(skillDir, e.ID+".json"), data, 0o644)
}

// applyMemoryToDisk memory 编辑：create=追加块；update/delete=按 [id] 块重写 memory.md
func applyMemoryToDisk(home string, e refineEdit, mode string) string {
	path := filepath.Join(home, "memory.md")
	orig := ""
	if data, err := os.ReadFile(path); err == nil {
		orig = string(data)
	}
	block := fmt.Sprintf("## [%s] %s\n%s", e.ID, e.Title, e.Content)
	switch mode {
	case "create":
		return block // 由调用方追加
	case "update":
		return replaceMemoryBlock(orig, e.ID, block)
	case "delete":
		return replaceMemoryBlock(orig, e.ID, "")
	}
	return orig
}

// replaceMemoryBlock 按 `## [id]` 开头块替换/删除
func replaceMemoryBlock(md, id, newBlock string) string {
	var keep []string
	inBlock := false
	replaced := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## [") && strings.Contains(line, "]" ) && strings.Contains(line, "## ["+id) {
			// 命中目标块：跳过原块
			inBlock = true
			if newBlock != "" && !replaced {
				keep = append(keep, newBlock)
				replaced = true
			}
			continue
		}
		if inBlock {
			if strings.HasPrefix(strings.TrimSpace(line), "##") {
				inBlock = false
				// 这个 line 是下一个块的开头，保留
				keep = append(keep, line)
				continue
			}
			continue // 本块内，丢弃
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n")
}

// applyRefinementProposal 校验 + 应用 edits + 记录事件（对应 prime 的 applyRefinementProposal）
func applyRefinementProposal(home string, state *harnessState, prop refineProposal, id string, scope string) refineResult {
	var applied []appliedRefineEdit
	for _, edit := range prop.Edits {
		resolvedID := edit.ID
		if resolvedID == "" {
			resolvedID = harnessSlug(edit.Title, "entry")
		}
		a := appliedRefineEdit{refineEdit: edit, ResolvedID: resolvedID}
		// 校验
		if edit.Action != refineCreate && edit.ID == "" {
			a.Error = string(edit.Action) + " 需要 id"; a.Applied = false; applied = append(applied, a); continue
		}
		if edit.Action != refineDelete && (edit.Title == "" || edit.Content == "") {
			a.Error = string(edit.Action) + " 需要 title 和 content"; a.Applied = false; applied = append(applied, a); continue
		}
		if edit.Kind == refineKindPrompt && (edit.ID == "base_system_prompt" || edit.Title == "base_system_prompt") {
			a.Error = "base system prompt 不可改（任何操作）"; a.Applied = false; applied = append(applied, a); continue
		}
		records := state.Entries[edit.Kind]
		before, exists := records[resolvedID]
		switch edit.Action {
		case refineCreate:
			if exists {
				a.Error = "entry 已存在（用 update）"; a.Applied = false; applied = append(applied, a); continue
			}
			now := time.Now()
			records[resolvedID] = harnessEntry{
				ID: resolvedID, Kind: edit.Kind, Title: edit.Title, Content: edit.Content,
				Scope: scope, Version: 1, CreatedAt: now, UpdatedAt: now,
			}
			// 物化到磁盘：技能→真实 skill JSON；记忆→memory.md
			if edit.Kind == refineKindSkill {
				applySkillToDisk(home, refineEdit{Action: edit.Action, Kind: edit.Kind, ID: resolvedID, Title: edit.Title, Content: edit.Content}, false)
			} else if edit.Kind == refineKindMemory {
				if block := applyMemoryToDisk(home, refineEdit{Action: edit.Action, Kind: edit.Kind, ID: resolvedID, Title: edit.Title, Content: edit.Content}, "create"); block != "" {
					appendFile(filepath.Join(home, "memory.md"), "\n"+block+"\n")
				}
			}
			a.Applied = true
		case refineUpdate:
			if !exists {
				a.Error = "entry 未找到"; a.Applied = false; applied = append(applied, a); continue
			}
			now := time.Now()
			upd := before
			upd.Title = edit.Title
			upd.Content = edit.Content
			upd.UpdatedAt = now
			upd.Version++
			upd.Scope = scope
			records[resolvedID] = upd
			if edit.Kind == refineKindSkill {
				applySkillToDisk(home, refineEdit{Action: edit.Action, Kind: edit.Kind, ID: resolvedID, Title: edit.Title, Content: edit.Content}, false)
			} else if edit.Kind == refineKindMemory {
				if md := applyMemoryToDisk(home, refineEdit{Action: edit.Action, Kind: edit.Kind, ID: resolvedID, Title: edit.Title, Content: edit.Content}, "update"); md != "" {
					os.WriteFile(filepath.Join(home, "memory.md"), []byte(strings.TrimPrefix(md, "\n")), 0o644)
				}
			}
			a.Applied = true
		case refineDelete:
			if !exists {
				a.Error = "entry 未找到"; a.Applied = false; applied = append(applied, a); continue
			}
			delete(records, resolvedID)
			if edit.Kind == refineKindSkill {
				applySkillToDisk(home, refineEdit{Action: edit.Action, Kind: edit.Kind, ID: resolvedID}, true)
			} else if edit.Kind == refineKindMemory {
				if md := applyMemoryToDisk(home, refineEdit{Action: edit.Action, Kind: edit.Kind, ID: resolvedID}, "delete"); md != "" {
					os.WriteFile(filepath.Join(home, "memory.md"), []byte(md), 0o644)
				}
			}
			a.Applied = true
		}
		applied = append(applied, a)
	}

	var changes []string
	for _, e := range applied {
		if e.Applied {
			changes = append(changes, fmt.Sprintf("%s %s:%s", e.Action, e.Kind, e.ResolvedID))
		}
	}
	state.Refinements = append(state.Refinements, harnessEvent{
		ID: id, Trigger: prop.Summary, Changes: changes, Outcome: prop.ExpectedOutcome, CreatedAt: time.Now(),
	})
	saveRefineState(home, state)
	res := refineResult{ID: id, Summary: prop.Summary, Rationale: prop.Rationale, Scope: scope, AppliedEdits: applied}
	appendRefineHistory(home, res)
	return res
}

// appendFile 追加文本到文件（不存在则创建）
func appendFile(path, text string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	f.WriteString(text)
	f.Close()
}

// ===== planRefinement：LLM 把轨迹提炼成 CRUD 提案（copy prime 的 REFINEMENT_SYSTEM_PROMPT 思路）=====

const refinementSystemPrompt = `你是 Rescene Agent 的「持续进化（refine）」子系统（抄自 Prime Agent 的 Continual Harness）。

你的工作：从最近的活动轨迹里，把可复用经验提炼成对「prompt笔记/记忆/技能/子代理规格」的精确增删改（CRUD）编辑。
记住：这不是总结对话，而是把轨迹里的经验落成下次能复用的可编辑状态。

四种组件分工：
- prompt：补充性行为准则笔记（仅补充）。基础 system/角色 prompt 不可改。
- memory：稳定的事实、决策、失败教训、偏好、结果。
- skill：可复现的流程/方法 → 写成技能（name/description/trigger/verification/steps 精髓）。
- subagent：可复用的「委派角色」规格（何时派给谁、目的、指令）。

范围与安全：
- 默认只做局部进化（本 agent 可复用即可）。跨会话稳定教训才值得存。
- 每条编辑要小而证据本位：没证据的直觉不要提。
- 基础 system/角色 prompt 绝对不可改写。不要直接改源码。
- 重复失败→技能/记忆；重复流程→技能；稳定事实→记忆；窄行为准则→prompt 笔记。
- 用 create 建新、用 update 改已有、用 delete 删废弃。尽量最小编辑。

当前可复用状态：
%s

最近 refine 历史：
%s

最近活动轨迹（live.log 尾部）：
%s

本次进化焦点（可为空）：
%s

只输出 JSON，不要任何解释和代码块：
{"summary":"一句话总结这次进化","rationale":"为什么这些编辑有轨迹证据支撑","expected_outcome":"期望改进什么、怎么验证","edits":[{"action":"create|update|delete","kind":"prompt|memory|skill|subagent","id":"稳定id(update/delete 必填)","title":"create/update 必填","content":"create/update 必填","reason":"这条为什么有用"}]}`

func planRefinement(home, focus string) (refineProposal, bool) {
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return refineProposal{}, false
	}
	state := loadRefineState(home)
	trajectory := liveLogTail(home, 60)
	history := refineHistoryPrompt(home)
	prompt := fmt.Sprintf(refinementSystemPrompt, harnessOverview(state), history, trajectory, focus)

	msg := ChatRequest{
		Model: model.Model, Messages: []ChatMessage{{Role: "user", Content: prompt}},
		Stream: false, MaxTokens: 2048, Temperature: 0.3,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	content, err := CompleteWithModel(ctx, model.ID, msg, nil)
	if err != nil {
		return refineProposal{}, false
	}
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	if end := strings.LastIndex(content, "}"); start != -1 && end > start {
		content = content[start : end+1]
	}
	var prop refineProposal
	if json.Unmarshal([]byte(content), &prop) != nil || prop.Summary == "" {
		return refineProposal{}, false
	}
	return prop, true
}

// refineHistoryPrompt 最近的 refine 历史（供模型参考避免重复）
func refineHistoryPrompt(home string) string {
	h := loadRefineHistory(home)
	if len(h) == 0 {
		return "（还没有过 refine）"
	}
	var parts []string
	for _, r := range h {
		if len(parts) >= 8 {
			break
		}
		parts = append(parts, fmt.Sprintf("- [%s] %s", r.ID, r.Summary))
	}
	return strings.Join(parts, "\n")
}

// ===== auto 门（copy prime reviewAutoRefine）：轻量判断这轮轨迹值不值得 refine，过滤噪音 =====

func refineGateShouldRun(home string) bool {
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return false
	}
	trajectory := liveLogTail(home, 30)
	prompt := fmt.Sprintf(`你是 refine 的自动评审门。判断下面这段最近轨迹，值不值得触发一次持续进化（refine）。
值得：出现重复失败、可复用战术、重复流程、稳定偏好、用户纠正。
不值得：一次性噪音、无证据的猜测、临时工具输出。
只输出 JSON：{"should_refine":true/false,"rationale":"短理由"}

最近轨迹：
%s`, trajectory)
	msg := ChatRequest{Model: model.Model, Messages: []ChatMessage{{Role: "user", Content: prompt}}, Stream: false, MaxTokens: 128, Temperature: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	content, err := CompleteWithModel(ctx, model.ID, msg, nil)
	if err != nil {
		return false
	}
	var v struct {
		ShouldRefine bool `json:"should_refine"`
	}
	json.Unmarshal([]byte(content), &v)
	return v.ShouldRefine
}

// ===== runRefine：executeTrumanAction 的 "refine" 执行器 =====
// 读轨迹 → LLM 提案 → 应用到可复用状态 → 返回人类可读摘要
func runRefine(d *Daughter, home, focus string) string {
	prop, ok := planRefinement(home, focus)
	if !ok {
		return ""
	}
	if len(prop.Edits) == 0 {
		return "（无编辑）"
	}
	state := loadRefineState(home)
	id := fmt.Sprintf("refine_%s", time.Now().Format("20060102150405"))
	res := applyRefinementProposal(home, state, prop, id, "local")
	var applied []string
	for _, e := range res.AppliedEdits {
		if e.Applied {
			applied = append(applied, fmt.Sprintf("%s:%s", e.Kind, e.ResolvedID))
		}
	}
	if len(applied) == 0 {
		return "（提案校验全部被拒）"
	}
	return fmt.Sprintf("%s — 应用 %d 条：%s", prop.Summary, len(applied), strings.Join(applied, ", "))
}

// rollbackRefine 逆向应用一次历史 refine（按 原始 edits 反向）
func rollbackRefine(home, targetID string) string {
	h := loadRefineHistory(home)
	var target *refineResult
	for i := range h {
		if h[i].ID == targetID {
			target = &h[i]
			break
		}
	}
	if target == nil {
		return fmt.Sprintf("refine %s 未找到", targetID)
	}
	state := loadRefineState(home)
	// 反向构造 inverse edits
	var inverse []refineEdit
	for i := len(target.AppliedEdits) - 1; i >= 0; i-- {
		e := target.AppliedEdits[i]
		if !e.Applied {
			continue
		}
		switch e.Action {
		case refineCreate:
			inverse = append(inverse, refineEdit{Action: refineDelete, Kind: e.Kind, ID: e.ResolvedID, Reason: "rollback"})
		case refineDelete, refineUpdate:
			// 尽力：create 回填一个空壳标记；真正的旧内容在精简版里没存，这里提示
			inverse = append(inverse, refineEdit{Action: refineCreate, Kind: e.Kind, ID: e.ResolvedID, Title: e.Title, Content: e.Content, Reason: "rollback(尽力回填)"})
		}
	}
	prop := refineProposal{Summary: "Rollback " + targetID, Rationale: "逆向应用失败改动", Edits: inverse}
	res := applyRefinementProposal(home, state, prop, targetID+"_rb", "local")
	var n int
	for _, e := range res.AppliedEdits {
		if e.Applied {
			n++
		}
	}
	return fmt.Sprintf("回滚 %s，应用 %d 条逆向编辑", targetID, n)
}

// ===== 进化统计：把 refine / 技能 / 记忆累积量化成「宝可梦」成长数据 =====

// runRefineCLI `rescene refine <agent家目录> [focus]` 手动触发一次进化；
// `rescene refine rollback <家目录> <refine_id>` 回滚一次进化；
// `rescene evolve <agent家目录>` 只查进化统计。
func runRefineCLI(args []string) {
	InitRouter() // 模型池必须先初始化（同 runCompany 的 2026-08-09 修复：漏了 → workingModels 空 → 所有模型动作失败）
	usage := "用法:\n  rescene refine <agent家目录> [focus]       # 触发一次持续进化\n  rescene refine rollback <家目录> <id>   # 回滚一次进化\n  rescene evolve <agent家目录>            # 查看进化统计（宝可梦量化）"
	if len(args) == 0 {
		fmt.Println(usage)
		return
	}
	if args[0] == "rollback" || args[0] == "rb" {
		if len(args) < 3 {
			fmt.Println("用法: rescene refine rollback <家目录> <refine_id>")
			return
		}
		fmt.Println(rollbackRefine(args[1], args[2]))
		return
	}
	home := args[0]
	if args[0] == "stat" || args[0] == "stats" {
		home = args[1]
	}
	if len(args) > 1 && (args[0] != "stat" && args[0] != "stats") {
		focus := strings.Join(args[1:], " ")
		fmt.Printf("🧬 触发 %s 的持续进化（focus: %s）…\n", home, runeClip(focus, 40))
		summary := runRefine(nil, home, focus)
		if summary == "" {
			fmt.Println("❌ 进化未完成（模型/解析失败，可能限流）")
			return
		}
		fmt.Printf("✅ 进化完成：%s\n", summary)
	}
	st := computeEvolutionStats(home)
	fmt.Printf("📊 进化统计：XP=%d Lv=%d 阶段=%s\n  技能=%d 记忆=%d 产出=%d refine=%d\n  六维雷达（0-100） 产出=%d 技能=%d 协作=%d 记忆=%d 成功率=%d 亲密度=%d\n",
		st.XP, st.Level, st.Stage, st.Skills, st.Memories, st.Outputs, st.Refines,
		st.Output, st.Skill, st.Collaboration, st.Memory, st.Success, st.Intimacy)
}

// evolutionStats 六维有效参数：每条都是真实运营参数（归一化 0-100 可比），不是虚构的 HP/攻/防
type evolutionStats struct {
	XP    int    `json:"xp"`
	Level int    `json:"level"`
	Stage string `json:"stage"`

	// 六维有效参数（雷达图，0-100）
	Output       int `json:"output"`       // 产出力：交付物数
	Skill         int `json:"skill"`        // 技能广度：技能库规模
	Collaboration int `json:"collab"`       // 协作度：真实交叉引用（接力他人+被接力）
	Memory        int `json:"memory"`       // 记忆沉淀：长期记忆块 + refine 记忆
	Success       int `json:"success"`      // 成功率：live.log 成功动作占比（真算）
	Intimacy      int `json:"intimacy"`     // 亲密度：memorydir 亲密等级（QQ 曲线 Lv）

	// 原始计数（详情/调试用）
	Refines  int `json:"refines"`
	Skills  int `json:"skills"`
	Memories int `json:"memories"`
	Outputs int `json:"outputs"`
}

// normScore 归一化到 0-100：min(100, value*scale)，value<0 归 0
func normScore(value, scale int) int {
	s := value * scale
	if s > 100 {
		return 100
	}
	if s < 0 {
		return 0
	}
	return s
}

func clampPct(v int) int {
	if v > 100 {
		return 100
	}
	if v < 0 {
		return 0
	}
	return v
}

var crossAgentRefRe = regexp.MustCompile(`\b(designer|writer|researcher|coder|promoter|publisher|ceo)-[0-9]{1,3}\b`)

// countCrossAgentRefs 统计 outputs/projects 里引用其它 agent 的次数（真实接力证据）
func countCrossAgentRefs(home string) int {
	count := 0
	scan := func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".md") && !strings.HasSuffix(p, ".txt") {
			return nil
		}
		data, e := os.ReadFile(p)
		if e != nil {
			return nil
		}
		count += len(crossAgentRefRe.FindAll(data, -1))
		return nil
	}
	filepath.WalkDir(filepath.Join(home, "outputs"), scan)
	filepath.WalkDir(filepath.Join(home, "projects"), scan)
	return count
}

// readIntimacy 读真实亲密度：~/rescene_data/memory/intimacy.md（memorydir 系统，云端 ResceneCloud 权威）
// 返回原始互动值 + 按 QQ 宠物曲线算出的 Lv（同 agent-growth-memory 亲密等级公式）
func readIntimacy() (value, level int) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return 0, 0
	}
	data, err := os.ReadFile(filepath.Join(hd, "rescene_data", "memory", "intimacy.md"))
	if err != nil {
		return 0, 0
	}
	s := strings.TrimSpace(string(data))
	if idx := strings.Index(s, ":"); idx != -1 {
		s = strings.TrimSpace(s[idx+1:])
	}
	value, _ = strconv.Atoi(s)
	if value > 0 {
		level = int((1 + sqrtf64(1+8*float64(value)/100)) / 2)
	}
	return value, level
}

// computeEvolutionStats 读取 home 真实文件算进化数值（六维全有磁盘/数据库证据）
func computeEvolutionStats(home string) evolutionStats {
	st := evolutionStats{}
	st.Refines = len(loadRefineHistory(home))
	if ents, err := os.ReadDir(filepath.Join(home, "skills")); err == nil {
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				st.Skills++
			}
		}
	}
	if mem, err := os.ReadFile(filepath.Join(home, "memory.md")); err == nil {
		st.Memories = strings.Count(string(mem), "## [") // 块标记计数（含文件开头的第一块）
	}
	if ents, err := os.ReadDir(filepath.Join(home, "outputs")); err == nil {
		st.Outputs = len(ents)
	}
	// 成功率：live.log 里 ✅/完成 vs ⚠️/失败 的真占比
	lines := liveLogTailLines(home, 99999)
	var ok, bad int
	for _, l := range lines {
		if strings.Contains(l, "✅") || strings.Contains(l, "完成") {
			ok++
		}
		if strings.Contains(l, "⚠️") || strings.Contains(l, "❌") || strings.Contains(l, "失败") || strings.Contains(l, "未") {
			bad++
		}
	}
	success := 0
	if ok+bad > 0 {
		success = int(float64(ok) / float64(ok+bad) * 100)
	}
	collab := countCrossAgentRefs(home)
	intimacyVal, intimacyLv := readIntimacy()

	// 六维（0-100，可比）
	st.Output        = normScore(st.Outputs, 10)
	st.Skill         = normScore(st.Skills, 10)
	st.Collaboration = normScore(collab, 15)
	st.Memory        = normScore(st.Memories+st.Refines, 10) // 记忆块 + 进化沉淀
	st.Success       = clampPct(success)
	st.Intimacy      = normScore(intimacyLv, 15) // Lv5=75, Lv7=100
	_ = intimacyVal

	// XP / 等级（训练成长感，不代表雷达轴）
	st.XP = st.Refines*30 + st.Skills*20 + st.Memories*10 + st.Outputs*5
	st.Level = int((1 + sqrtf64(1+8*float64(st.XP)/100)) / 2)
	st.Stage = evolutionStage(st)
	return st
}

// evolutionStage 进化阶段判定（赛博成长链：静默电流→数据尘埃→神经漫游者→矩阵幽灵→冬寂）
func evolutionStage(st evolutionStats) string {
	switch {
	case st.Skills >= 20 && st.Memories >= 15:
		return "Lv.5 冬寂"
	case st.Skills >= 10 && st.Memories >= 8:
		return "Lv.4 矩阵幽灵"
	case st.Skills >= 5 && st.Memories >= 4:
		return "Lv.3 神经漫游者"
	case st.Skills >= 2:
		return "Lv.2 数据尘埃"
	default:
		return "Lv.1 静默电流"
	}
}

func sqrtf64(v float64) float64 {
	// 无需 math import 的手写：牛顿法一两步足够判等级
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 6; i++ {
		x = (x + v/x) / 2
	}
	return x
}

// loadSkillsForHome 决策视图：共享技能库 + 本 agent 的 skills/（refine 物化的技能能被子决策看到）
func loadSkillsForHome(home string) []Skill {
	merged := append([]Skill{}, loadSkills()...)
	seen := map[string]bool{}
	for _, s := range merged {
		seen[s.Name] = true
	}
	if ents, err := os.ReadDir(filepath.Join(home, "skills")); err == nil {
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(home, "skills", e.Name()))
			if err != nil {
				continue
			}
			var s Skill
			if json.Unmarshal(data, &s) != nil || s.Name == "" || seen[s.Name] {
				continue
			}
			merged = append(merged, s)
			seen[s.Name] = true
		}
	}
	return merged
}