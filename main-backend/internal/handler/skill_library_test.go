package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 所有用例都把 AURORA_SKILLS_DIR 指到临时目录，绝不碰用户真实的 ./skills。
func withTempSkillsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AURORA_SKILLS_DIR", dir)
	return dir
}

func writeSkillFile(t *testing.T, dir string, s Skill) {
	t.Helper()
	data, _ := json.Marshal(s)
	if err := os.WriteFile(filepath.Join(dir, s.Name+".json"), data, 0644); err != nil {
		t.Fatalf("写入技能文件失败: %v", err)
	}
}

// 常驻工具集必须包含 read_skill 本身——否则模型没有任何办法把技能全文取回来，
// 技能库的索引就只是摆设。
func TestNativeToolsAlwaysIncludeReadSkill(t *testing.T) {
	tools := buildCodeWorkflowTools(nil)
	found := false
	for _, tl := range tools {
		fn := tl["function"].(map[string]any)
		if fn["name"] == readSkillToolName {
			found = true
		}
	}
	if !found {
		t.Fatalf("常驻工具集里没有 %s，技能全文将永远不可达", readSkillToolName)
	}
}

// 核心诉求：read_skill 必须能拿到 steps——这是过去 skillLibraryPrompt 里
// 被解析出来又直接丢弃、模型永远拿不到的那部分内容。
func TestHandleReadSkillReturnsSteps(t *testing.T) {
	dir := withTempSkillsDir(t)
	writeSkillFile(t, dir, Skill{
		Name: "deploy-frontend", Description: "部署前端到生产环境",
		Steps: []string{"运行 npm build", "上传 dist 到 CDN", "刷新缓存"},
	})

	out := handleReadSkill(`{"names":["deploy-frontend"]}`, loadSkills())
	if !strings.Contains(out, "运行 npm build") || !strings.Contains(out, "刷新缓存") {
		t.Fatalf("read_skill 结果里没有完整 steps，实得: %s", out)
	}
}

func TestHandleReadSkillUnknownName(t *testing.T) {
	withTempSkillsDir(t)
	out := handleReadSkill(`{"names":["不存在的技能"]}`, loadSkills())
	if !strings.Contains(out, "不存在") {
		t.Errorf("应提示名字不存在让模型自己纠正，实得: %s", out)
	}
}

func TestHandleReadSkillBadArgs(t *testing.T) {
	withTempSkillsDir(t)
	if out := handleReadSkill(`{`, nil); !strings.Contains(out, "解析失败") {
		t.Errorf("坏 JSON 应回可读提示而不是空串: %q", out)
	}
	if out := handleReadSkill(`{"names":[]}`, nil); !strings.Contains(out, "为空") {
		t.Errorf("空 names 应回提示: %q", out)
	}
}

// 已知+未知名字混在一次调用里，应该各自正确归类，互不影响。
func TestHandleReadSkillMixedNames(t *testing.T) {
	dir := withTempSkillsDir(t)
	writeSkillFile(t, dir, Skill{Name: "a-skill", Description: "d1", Steps: []string{"s1"}})

	out := handleReadSkill(`{"names":["a-skill","b-skill"]}`, loadSkills())
	if !strings.Contains(out, "s1") {
		t.Errorf("已知技能 a-skill 的步骤应出现在结果里: %s", out)
	}
	if !strings.Contains(out, "b-skill") || !strings.Contains(out, "不存在") {
		t.Errorf("未知技能 b-skill 应出现在'不存在'提示里: %s", out)
	}
}

// 索引（skillLibraryPrompt）依然只给名称+描述，不该把 steps 也塞进去——
// 否则又变回一次性全量注入，read_skill 的按需加载就没有意义了。
func TestSkillLibraryPromptIsIndexOnly(t *testing.T) {
	dir := withTempSkillsDir(t)
	writeSkillFile(t, dir, Skill{
		Name: "some-skill", Description: "一句话描述",
		Steps: []string{"这一步不该出现在索引里的独特字符串ZZZ"},
	})

	prompt := skillLibraryPrompt()
	if !strings.Contains(prompt, "some-skill") || !strings.Contains(prompt, "一句话描述") {
		t.Errorf("索引里应有名称+描述，实得: %s", prompt)
	}
	if strings.Contains(prompt, "ZZZ") {
		t.Errorf("索引不该包含 steps 正文，否则按需加载失去意义: %s", prompt)
	}
}
