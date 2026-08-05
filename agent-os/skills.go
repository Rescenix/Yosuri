package main

// skills.go — 技能沉淀（移植自主应用 skill_library.go 的 generateSkillAsync 简化版）
//
// 工作流成功收尾后，异步把本次动作序列抽象成一个可复用技能（JSON），
// 写入用户技能库 ~/rescene_data/skills/（与网页端共用同一技能库，
// CLI 沉淀的技能网页端也能读，反之亦然）；
// 下次启动时技能索引注入系统提示词，让 Agent 知道"这类任务以前是怎么做成的"。
//
// 与主应用的区别：CLI 只保留核心闭环（沉淀 + 加载 + 注入），
// 不引入 builtin/external 来源、归档 GC 与设置页治理——那些是 GUI 的职责。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Skill 技能结构（与主应用 Skill 兼容的字段子集，GUI 可直接读取）
type Skill struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Steps        []string  `json:"steps,omitempty"`
	Trigger      string    `json:"trigger,omitempty"`
	Verification string    `json:"verification,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	UseCount     int       `json:"use_count,omitempty"`
}

// skillsDir 技能库目录：与主应用共用 ~/rescene_data/skills
func skillsDir() string {
	if dir := os.Getenv("RESCENE_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "skills")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "rescene_data", "skills")
	}
	return filepath.Join(home, "rescene_data", "skills")
}

// loadSkills 读取技能库全部技能（自研沉淀 JSON）
func loadSkills() []Skill {
	entries, err := os.ReadDir(skillsDir())
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(skillsDir(), e.Name()))
		if err != nil {
			continue
		}
		var s Skill
		if json.Unmarshal(data, &s) != nil || s.Name == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// skillsIndexPrompt 技能索引（注入系统提示词）——名字+描述一行一个
func skillsIndexPrompt() string {
	skills := loadSkills()
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n已沉淀技能库（任务与以下技能相关时，先参考它们的方法再动手）：\n")
	for _, s := range skills {
		fmt.Fprintf(&sb, "- %s：%s\n", s.Name, s.Description)
	}
	return sb.String()
}

var skillNameSanitizer = regexp.MustCompile(`[^a-z0-9\-]+`)

// generateSkill 工作流成功后异步沉淀技能。失败只打一行日志，绝不影响主流程。
func generateSkill(task string, transcript []string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("⚠️ 技能生成异常: %v\n", r)
		}
	}()

	// 动作序列太少（纯问答/简单任务）不值得沉淀
	if len(transcript) < 3 {
		return
	}

	prompt := fmt.Sprintf(`以下是一次成功完成的 Agent 任务的动作记录。
如果这个工作流对未来同类任务有复用价值，把它抽象成一个技能；如果只是一次性的琐碎操作，输出 {"name":""}。

任务：%s

动作序列：
%s

只输出一个 JSON 对象，不要任何解释和代码块包裹。技能必须有明确触发条件、可验证结果，且只保留 3–6 个可执行步骤；不得写入临时路径、密钥或项目专属垃圾信息：
{"name":"kebab-case英文技能名","description":"一句话中文描述什么场景用这个技能","trigger":"何时调用","verification":"如何验证成功","steps":["步骤1","步骤2","步骤3"]}`,
		runeClip(task, 500), strings.Join(transcript, "\n"))

	msg := ChatRequest{
		Model:       currentModel,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false, // 非流式：后台任务不往终端打内容
		MaxTokens:   2048,
		Temperature: 0.3,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	content, err := CompleteWithModel(ctx, msg.Model, msg, nil)
	if err != nil {
		fmt.Printf("⚠️ 技能生成调用失败: %v\n", err)
		return
	}

	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var skill Skill
	if err := json.Unmarshal([]byte(content), &skill); err != nil {
		fmt.Printf("⚠️ 技能 JSON 解析失败: %v\n", err)
		return
	}
	skill.Name = skillNameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(skill.Name)), "-")
	skill.Name = strings.Trim(skill.Name, "-")
	// 质量门槛：模型判定无复用价值则静默放弃
	if skill.Name == "" || len(skill.Steps) < 3 || len(skill.Steps) > 6 || skill.Trigger == "" || skill.Verification == "" {
		return
	}

	if err := os.MkdirAll(skillsDir(), 0o755); err != nil {
		return
	}
	skill.CreatedAt = time.Now()
	skill.UpdatedAt = skill.CreatedAt
	path := filepath.Join(skillsDir(), skill.Name+".json")
	data, _ := json.MarshalIndent(skill, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Printf("⚠️ 写入技能文件失败: %v\n", err)
		return
	}
	fmt.Printf(ColorGreen+"🎓 新技能已自动沉淀: %s（%s）"+ColorReset+"\n", skill.Name, skill.Description)
}
