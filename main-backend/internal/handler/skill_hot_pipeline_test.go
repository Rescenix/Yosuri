package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 热集管线测试全部指向临时目录，绝不碰用户真实 ~/rescene_data/skills 的账本。
func withTempHotSkillDirs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AURORA_SKILLS_DIR", dir)
	t.Setenv("AURORA_EXT_SKILLS_DIR", filepath.Join(dir, "ext-empty"))
	return dir
}

func seedLedger(t *testing.T, dir string, name string, ages ...time.Duration) {
	t.Helper()
	now := time.Now()
	l := loadSkillLedger() // 合并写：多次调用累积不同技能，互不覆盖
	entries := l[name]
	for _, a := range ages {
		entries = append(entries, now.Add(-a).Unix())
	}
	l[name] = entries
	data, _ := json.Marshal(l)
	_ = os.MkdirAll(dir, 0755)
	if err := os.WriteFile(filepath.Join(dir, ".usage.json"), data, 0644); err != nil {
		t.Fatalf("写账本失败: %v", err)
	}
}

func hotTestSkill(name, body string) Skill {
	return Skill{Name: name, Description: "测试技能 " + name, Source: "learned",
		Status: skillStatusActive, Body: body, Steps: []string{"步骤一", "步骤二", "步骤三"}}
}

// 冷启动：账本为空 → 热集缺席，整体退化为纯索引模式。
func TestHotPipelineColdStartIsEmpty(t *testing.T) {
	withTempHotSkillDirs(t)
	section, st := hotSkillsPrompt([]Skill{hotTestSkill("a-skill", "A的正文")})
	if section != "" || st.Name != "" {
		t.Fatalf("冷启动不应有热集，实得 section=%q state=%+v", section, st)
	}
}

// 使用次数过阈值 → 入选，正文进渲染段。
func TestHotPipelineInjectsHotSkillBody(t *testing.T) {
	dir := withTempHotSkillDirs(t)
	seedLedger(t, dir, "a-skill", time.Hour, 2*time.Hour, 3*time.Hour, 4*time.Hour)
	section, st := hotSkillsPrompt([]Skill{hotTestSkill("a-skill", "A的正文标记ABC")})
	if st.Name != "a-skill" || !strings.Contains(section, "A的正文标记ABC") {
		t.Fatalf("4 次近期使用应入选，实得 state=%+v section=%q", st, section)
	}
	if !strings.Contains(section, "步骤一") {
		t.Fatalf("steps 应随正文一起渲染")
	}
}

// 不够热（1 次使用）不入选。
func TestHotPipelineBelowThresholdStaysOut(t *testing.T) {
	dir := withTempHotSkillDirs(t)
	seedLedger(t, dir, "a-skill", time.Hour)
	section, _ := hotSkillsPrompt([]Skill{hotTestSkill("a-skill", "A的正文")})
	if section != "" {
		t.Fatalf("1 次使用不应入选")
	}
}

// 衰减：3 次使用但都在 90 天前 → 分数掉到阈值下，自然冷却。
func TestHotPipelineDecaysOut(t *testing.T) {
	dir := withTempHotSkillDirs(t)
	seedLedger(t, dir, "a-skill", 90*24*time.Hour, 91*24*time.Hour, 92*24*time.Hour)
	section, _ := hotSkillsPrompt([]Skill{hotTestSkill("a-skill", "A的正文")})
	if section != "" {
		t.Fatalf("90 天前的使用应衰减到阈值下")
	}
}

// 滞回：在位技能分数 3.15，挑战者 4.0（不足 2 倍）→ 不换人。
func TestHotPipelineHysteresisKeepsIncumbent(t *testing.T) {
	dir := withTempHotSkillDirs(t)
	skills := []Skill{hotTestSkill("a-skill", "A正文"), hotTestSkill("b-skill", "B正文")}
	// 第一轮：b 热（5 次×20 天前≈3.15），a 冷 → b 成为在位
	seedLedger(t, dir, "b-skill", 20*24*time.Hour, 20*24*time.Hour, 20*24*time.Hour, 20*24*time.Hour, 20*24*time.Hour)
	seedLedger(t, dir, "a-skill", time.Hour)
	_, st := hotSkillsPrompt(skills)
	if st.Name != "b-skill" {
		t.Fatalf("首轮应选 b-skill，实得 %+v", st)
	}
	// 第二轮：a 变热（4 次×1 天内≈4.0 > 3.15）但不足 b 的 2 倍 → b 留任
	seedLedger(t, dir, "a-skill", time.Hour, 2*time.Hour, 3*time.Hour, 4*time.Hour)
	_, st2 := hotSkillsPrompt(skills)
	if st2.Name != "b-skill" {
		t.Fatalf("滞回应让在位 b-skill 留任，实得 %+v", st2)
	}
}

// 滞回的另一面：挑战者分数 ≥ 在位 2 倍 → 换人。
func TestHotPipelineHysteresisAllowsSwap(t *testing.T) {
	dir := withTempHotSkillDirs(t)
	skills := []Skill{hotTestSkill("a-skill", "A正文"), hotTestSkill("b-skill", "B正文")}
	seedLedger(t, dir, "b-skill", 20*24*time.Hour, 20*24*time.Hour, 20*24*time.Hour, 20*24*time.Hour, 20*24*time.Hour)
	_, st := hotSkillsPrompt(skills) // b 在位（≈3.15）
	if st.Name != "b-skill" {
		t.Fatalf("前置条件：b 应先入选，实得 %+v", st)
	}
	// a 冲到 8 次近期使用（≈8.0 ≥ 3.15×2）→ 换人
	seedLedger(t, dir, "a-skill", time.Hour, 2*time.Hour, 3*time.Hour, 4*time.Hour, 5*time.Hour, 6*time.Hour, 7*time.Hour, 8*time.Hour)
	_, st2 := hotSkillsPrompt(skills)
	if st2.Name != "a-skill" {
		t.Fatalf("a 分数超 b 的 2 倍，应换人为 a-skill，实得 %+v", st2)
	}
}

// 在位技能被删除 → 位置作废，不渲染幽灵正文。
func TestHotPipelineIncumbentDeleted(t *testing.T) {
	dir := withTempHotSkillDirs(t)
	seedLedger(t, dir, "gone-skill", time.Hour, 2*time.Hour, 3*time.Hour, 4*time.Hour)
	saveHotSkillState(hotSkillState{Name: "gone-skill", Score: 3, At: time.Now()})
	section, st := hotSkillsPrompt([]Skill{hotTestSkill("other-skill", "别的正文")})
	if st.Name == "gone-skill" || strings.Contains(section, "gone") {
		t.Fatalf("已删除技能不应留在热集，实得 %+v", st)
	}
}

// 正文超长截断，防单个热技能撑爆上下文。
func TestHotPipelineBodyTruncated(t *testing.T) {
	dir := withTempHotSkillDirs(t)
	seedLedger(t, dir, "big-skill", time.Hour, 2*time.Hour, 3*time.Hour, 4*time.Hour)
	huge := strings.Repeat("很长很长的正文", hotSkillBodyCap)
	section, _ := hotSkillsPrompt([]Skill{hotTestSkill("big-skill", huge)})
	if !strings.Contains(section, "已截断") {
		t.Fatalf("超长正文应截断")
	}
	if len([]rune(section)) > hotSkillBodyCap+2000 {
		t.Fatalf("截断后仍过大: %d runes", len([]rune(section)))
	}
}

// skill_view 取全文要记进账本（所有来源，含 builtin）。
func TestSkillViewRecordsLedgerUse(t *testing.T) {
	withTempHotSkillDirs(t)
	skills := []Skill{{Name: "builtin-x", Description: "d", Source: "builtin", Steps: []string{"a", "b", "c"}}}
	handleSkillView(`{"name":"builtin-x"}`, skills)
	l := loadSkillLedger()
	if len(l["builtin-x"]) != 1 {
		t.Fatalf("skill_view 应记一次使用，实得 %+v", l)
	}
}

// provider 收尾补记：RecordHotSkillUse 只在有热集时写账本。
func TestRecordHotSkillUseOnlyWhenHot(t *testing.T) {
	withTempHotSkillDirs(t)
	p := &contextProvider{}
	p.RecordHotSkillUse()
	if len(loadSkillLedger()) != 0 {
		t.Fatalf("无热集不应记账")
	}
	p.hotSkillName = "hot-one"
	p.RecordHotSkillUse()
	if len(loadSkillLedger()["hot-one"]) != 1 {
		t.Fatalf("有热集应补记一次使用")
	}
}
