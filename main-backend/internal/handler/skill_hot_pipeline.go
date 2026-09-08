package handler

// 热门技能动态管线 —— 用「真实使用频率」决定谁常驻上下文。
//
// 与已删除的 autoLoadedSkillsPrompt 的区别：旧管线拿任务文本猜 trigger 关键词
// （静态、脆、误召回）；这里按行为证据打分——模型每次 skill_view 取全文就记一次
// 使用，衰减频率最高的技能自动获得全文预加载资格。冷启动时账本为空，热集为空，
// 整体退化为纯索引模式，零风险。
//
// 三层结构：索引（人人常驻）→ 热集（频率自动电梯，本文件）→ skill_view（手动楼梯）。

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// hotSkillHalfLife 衰减半衰期：30 天前的使用只算一半权重。
const hotSkillHalfLife = 30 * 24 * time.Hour

// hotSkillMinScore 准入阈值：30 天内约 3 次使用才够热（宁缺毋滥）。
const hotSkillMinScore = 3.0

// hotSkillHysteresis 滞回系数：在位技能要被超过 2 倍分才换人，防两个技能反复换、缓存全碎。
const hotSkillHysteresis = 2.0

// hotSkillLedgerCap 每个技能最多保留的使用时间戳数（防账本无限膨胀）。
const hotSkillLedgerCap = 40

// skillUsageLedger 使用账本：技能名 → 使用时间戳（unix 秒，升序）。
// 与技能 JSON 文件分开存，因此 builtin/external 技能（不可写回自身文件）也能被追踪。
type skillUsageLedger map[string][]int64

func skillLedgerPath() string {
	return filepath.Join(skillsDir(), ".usage.json")
}

func hotSkillStatePath() string {
	return filepath.Join(skillsDir(), ".hot.json")
}

// loadSkillLedger 读账本；文件不存在/损坏返回空账本（绝不因统计失败影响主流程）。
func loadSkillLedger() skillUsageLedger {
	data, err := os.ReadFile(skillLedgerPath())
	if err != nil {
		return skillUsageLedger{}
	}
	var l skillUsageLedger
	if json.Unmarshal(data, &l) != nil {
		return skillUsageLedger{}
	}
	return l
}

func saveSkillLedger(l skillUsageLedger) {
	data, err := json.Marshal(l)
	if err != nil {
		return
	}
	_ = os.MkdirAll(skillsDir(), 0755)
	_ = os.WriteFile(skillLedgerPath(), data, 0644)
}

// recordSkillUse 记一次使用。所有来源的技能都记（账本独立于技能文件本身）。
func recordSkillUse(name string) {
	if name == "" {
		return
	}
	l := loadSkillLedger()
	now := time.Now().Unix()
	entries := append(l[name], now)
	if len(entries) > hotSkillLedgerCap {
		entries = entries[len(entries)-hotSkillLedgerCap:]
	}
	l[name] = entries
	saveSkillLedger(l)
}

// hotSkillScore 衰减频率分：Σ 0.5^(age/半衰期)。只统计账本内的近期使用。
func hotSkillScore(name string, l skillUsageLedger, now time.Time) float64 {
	score := 0.0
	for _, ts := range l[name] {
		age := now.Sub(time.Unix(ts, 0))
		if age < 0 {
			age = 0
		}
		score += math.Pow(0.5, float64(age)/float64(hotSkillHalfLife))
	}
	return score
}

// hotSkillState 在位热技能（滞回用）。落盘是因为热集跨进程也该稳定。
type hotSkillState struct {
	Name  string    `json:"name"`
	Score float64   `json:"score"`
	At    time.Time `json:"at"`
}

func loadHotSkillState() hotSkillState {
	var st hotSkillState
	data, err := os.ReadFile(hotSkillStatePath())
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, &st)
	return st
}

func saveHotSkillState(st hotSkillState) {
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(skillsDir(), 0755)
	_ = os.WriteFile(hotSkillStatePath(), data, 0644)
}

// pickHotSkill 选出本轮热集（top-1）：
//   - 候选 = 技能库里存在、且分数过阈值的技能
//   - 滞回：在位技能只要仍达标，就要求挑战者分数 ≥ 在位分×hysteresis 才换人
//   - 冷启动（账本空）返回空串 → 热集段整体缺席，退化为纯索引模式
func pickHotSkill(skills []Skill) (Skill, hotSkillState) {
	now := time.Now()
	l := loadSkillLedger()
	byName := map[string]Skill{}
	for _, s := range skills {
		byName[s.Name] = s
	}
	st := loadHotSkillState()
	// 在位技能已从库里消失（删除/归档）→ 位置作废，并清掉盘上的幽灵状态
	if st.Name != "" {
		if _, ok := byName[st.Name]; !ok {
			st = hotSkillState{}
			saveHotSkillState(st)
		}
	}
	var best Skill
	bestScore := 0.0
	for _, s := range skills {
		score := hotSkillScore(s.Name, l, now)
		if score < hotSkillMinScore {
			continue
		}
		if score > bestScore {
			best, bestScore = s, score
		}
	}
	if st.Name != "" && best.Name != st.Name {
		incumbent := hotSkillScore(st.Name, l, now)
		if incumbent >= hotSkillMinScore && bestScore < incumbent*hotSkillHysteresis {
			// 挑战者不够格，在位者留任
			best, bestScore = byName[st.Name], incumbent
		}
	}
	if best.Name == "" {
		return Skill{}, hotSkillState{}
	}
	return best, hotSkillState{Name: best.Name, Score: bestScore, At: now}
}

// hotSkillBodyCap 预加载正文上限：再热的技能也不许把上下文撑爆。
const hotSkillBodyCap = 6000

// hotSkillsPrompt 渲染热集段（易变段，排在 memory 前）。无热技能返回空串。
// 选定即落盘在位状态（滞回跨进程稳定）。
func hotSkillsPrompt(skills []Skill) (string, hotSkillState) {
	skill, st := pickHotSkill(skills)
	if skill.Name == "" {
		return "", hotSkillState{}
	}
	saveHotSkillState(st)
	var b strings.Builder
	b.WriteString("\n━━━ 热门技能（近期高频使用，宿主已自动加载全文） ━━━\n")
	b.WriteString("以下内容已随本轮上下文加载，直接遵循即可，不必再调 skill_view 重复取回。\n")
	fmt.Fprintf(&b, "\n## %s\n用途：%s\n", skill.Name, skill.Description)
	if skill.Trigger != "" {
		fmt.Fprintf(&b, "触发条件：%s\n", skill.Trigger)
	}
	if skill.Verification != "" {
		fmt.Fprintf(&b, "验证方式：%s\n", skill.Verification)
	}
	if len(skill.Steps) > 0 {
		b.WriteString("执行步骤：\n")
		for i, step := range skill.Steps {
			fmt.Fprintf(&b, "%d. %s\n", i+1, step)
		}
	}
	if skill.Body != "" {
		body := skill.Body
		if len([]rune(body)) > hotSkillBodyCap {
			body = string([]rune(body)[:hotSkillBodyCap]) + "\n…（正文过长已截断，完整内容用 skill_view 取回）"
		}
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String(), st
}
