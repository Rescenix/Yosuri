package handler

// 外部技能（Anthropic/Claude 风格 SKILL.md）加载 + 与自研沉淀技能区分的测试。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 把外部技能目录指到临时目录，写一个子目录 SKILL.md，绝不碰真实 ./skills-ext。
func withTempExtSkillsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AURORA_EXT_SKILLS_DIR", dir)
	return dir
}

func writeExtSkill(t *testing.T, root, name, md string) {
	t.Helper()
	sub := filepath.Join(root, name)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("建外部技能目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatalf("写 SKILL.md 失败: %v", err)
	}
}

func TestParseSkillMD(t *testing.T) {
	name, desc, body := parseSkillMD("---\nname: frontend-design\ndescription: 前端设计原则\n---\n\n# 正文\n内容行")
	if name != "frontend-design" || desc != "前端设计原则" {
		t.Fatalf("frontmatter 解析错误: name=%q desc=%q", name, desc)
	}
	if !strings.HasPrefix(body, "# 正文") {
		t.Fatalf("正文提取错误: %q", body)
	}
	// 带引号的 description 要去引号
	_, d2, _ := parseSkillMD("---\ndescription: \"引号包裹\"\n---\nx")
	if d2 != "引号包裹" {
		t.Fatalf("引号未去除: %q", d2)
	}
	// 没有 frontmatter：整篇当正文，name 空
	n3, _, b3 := parseSkillMD("# 无 frontmatter\n只有正文")
	if n3 != "" || !strings.Contains(b3, "只有正文") {
		t.Fatalf("无 frontmatter 处理错误: name=%q body=%q", n3, b3)
	}
}

func TestLoadExternalSkillsAndSource(t *testing.T) {
	learnedDir := withTempSkillsDir(t)     // 复用现有 helper（skill_library_test.go）
	extDir := withTempExtSkillsDir(t)

	writeSkillFile(t, learnedDir, Skill{Name: "deploy-frontend", Description: "部署前端", Steps: []string{"a", "b"}})
	writeExtSkill(t, extDir, "frontend-design", "---\nname: frontend-design\ndescription: 前端设计原则\n---\n\n正文文档内容")

	all := loadSkills()
	var learned, external *Skill
	for i := range all {
		switch all[i].Name {
		case "deploy-frontend":
			learned = &all[i]
		case "frontend-design":
			external = &all[i]
		}
	}
	if learned == nil || external == nil {
		t.Fatalf("两类技能未同时加载: %+v", all)
	}
	if learned.Source != "learned" {
		t.Errorf("自研技能 source 应为 learned，得到 %q", learned.Source)
	}
	if external.Source != "external" {
		t.Errorf("外部技能 source 应为 external，得到 %q", external.Source)
	}
	if len(external.Steps) != 0 || external.Body == "" {
		t.Errorf("外部技能应有 Body 无 Steps: %+v", external)
	}
}

// skill_view 对外部技能要回 content（正文）+ linked_files（关联文件），对自研技能回 steps。
func TestHandleSkillViewExternalReturnsContent(t *testing.T) {
	withTempSkillsDir(t)
	extDir := withTempExtSkillsDir(t)
	writeExtSkill(t, extDir, "frontend-design", "---\nname: frontend-design\ndescription: 前端设计原则\n---\n\n这里是完整说明文档")

	out := handleSkillView(`{"names":["frontend-design"]}`, loadSkills())
	if !strings.Contains(out, "这里是完整说明文档") {
		t.Fatalf("skill_view 未回外部技能正文: %s", out)
	}
	if !strings.Contains(out, "content") {
		t.Fatalf("外部技能结果应带 content 字段: %s", out)
	}
}

// 外部技能目录下带 tokens/ 等关联子目录时，skill_view 要列出 linked_files。
func TestHandleSkillViewExternalLinkedFiles(t *testing.T) {
	withTempSkillsDir(t)
	extDir := withTempExtSkillsDir(t)
	sub := filepath.Join(extDir, "frontend-design", "tokens")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("建 tokens 子目录失败: %v", err)
	}
	writeExtSkill(t, extDir, "frontend-design", "---\nname: frontend-design\ndescription: 前端设计原则\n---\n正文")
	if err := os.WriteFile(filepath.Join(sub, "INDEX.md"), []byte("# 色板索引\n"), 0o644); err != nil {
		t.Fatalf("写关联文件失败: %v", err)
	}

	out := handleSkillView(`{"names":["frontend-design"]}`, loadSkills())
	if !strings.Contains(out, "linked_files") || !strings.Contains(out, "tokens/INDEX.md") {
		t.Fatalf("skill_view 未列出关联文件: %s", out)
	}
}

// skill_view 带 name + file_path 时直接返回关联文件正文，越界路径被拒绝。
func TestHandleSkillViewReadLinkedFile(t *testing.T) {
	withTempSkillsDir(t)
	extDir := withTempExtSkillsDir(t)
	sub := filepath.Join(extDir, "frontend-design", "tokens")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("建 tokens 子目录失败: %v", err)
	}
	writeExtSkill(t, extDir, "frontend-design", "---\nname: frontend-design\ndescription: 前端设计原则\n---\n正文")
	if err := os.WriteFile(filepath.Join(sub, "stripe.tokens.md"), []byte("Stripe 色板：#635BFF\n"), 0o644); err != nil {
		t.Fatalf("写关联文件失败: %v", err)
	}

	out := handleSkillView(`{"name":"frontend-design","file_path":"tokens/stripe.tokens.md"}`, loadSkills())
	if !strings.Contains(out, "#635BFF") {
		t.Fatalf("skill_view 未回关联文件正文: %s", out)
	}

	// 越界路径必须被拒
	if bad := handleSkillView(`{"name":"frontend-design","file_path":"../../etc/passwd"}`, loadSkills()); !strings.Contains(bad, "越界") {
		t.Fatalf("越界关联文件路径未被拒绝: %s", bad)
	}
}

// 索引提示词要给外部技能打 [外部] 标，让模型知道那是文档型技能。
func TestSkillPromptTagsExternal(t *testing.T) {
	withTempSkillsDir(t)
	extDir := withTempExtSkillsDir(t)
	writeExtSkill(t, extDir, "frontend-design", "---\nname: frontend-design\ndescription: 前端设计原则\n---\n正文")

	prompt := skillLibraryPrompt()
	if !strings.Contains(prompt, "[外部]") || !strings.Contains(prompt, "frontend-design") {
		t.Fatalf("索引未给外部技能打标: %s", prompt)
	}
}
