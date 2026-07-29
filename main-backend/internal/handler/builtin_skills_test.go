package handler

import "testing"

func TestCuratedSkillsAreEmbedded(t *testing.T) {
	got := map[string]Skill{}
	for _, skill := range loadBuiltinSkills() {
		got[skill.Name] = skill
		if skill.Source != "builtin" || skill.Status != skillStatusActive {
			t.Errorf("%s 的内置标记不正确: source=%s status=%s", skill.Name, skill.Source, skill.Status)
		}
	}
	for _, name := range []string{
		"code-review-keys-and-binaries",
		"create-vue3-base-component",
		"frontend-design",
		"frontend-self-check",
	} {
		if _, ok := got[name]; !ok {
			t.Errorf("发布包漏掉内置技能 %s", name)
		}
	}
	if len(got) != 4 {
		t.Errorf("应只内置 4 个精选技能，实得 %d: %v", len(got), got)
	}
	if skill := got["frontend-design"]; skill.Body == "" {
		t.Error("frontend-design 的完整设计规范未随发布包内置")
	}
}
