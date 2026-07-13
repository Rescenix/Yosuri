package handler

// 技能库 —— 仿 Hermes 的闭环学习。
//
// 工作流成功收尾后，异步把这次的动作序列抽象成一个可复用技能（JSON 文件），
// 存入本地技能库目录；下次工作流启动时，技能库的名称+描述会注入系统提示词，
// 让 Agent 知道"这类任务以前是怎么做成的"。
//
// 目录：AURORA_SKILLS_DIR 环境变量，默认 ./skills（相对 server 工作目录）。

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

func skillsDir() string {
	if dir := os.Getenv("AURORA_SKILLS_DIR"); dir != "" {
		return dir
	}
	return "./skills"
}

// skillLibraryPrompt 把技能库整理成系统提示词片段；库为空时返回空串。
// 只注入名称+描述，正文步骤不进上下文（token 是成本），Agent 需要时靠描述回忆做法。
func skillLibraryPrompt() string {
	entries, err := os.ReadDir(skillsDir())
	if err != nil {
		return ""
	}
	var lines []string
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
		lines = append(lines, fmt.Sprintf("- %s：%s", s.Name, s.Description))
	}
	if len(lines) == 0 {
		return ""
	}
	sort.Strings(lines)
	return "\n━━━ 技能库（以往任务沉淀的成功做法，仅供参考） ━━━\n" + strings.Join(lines, "\n") + "\n"
}

var skillNameSanitizer = regexp.MustCompile(`[^a-z0-9\-]+`)

// generateSkillAsync 在工作流成功后异步抽象技能。失败只打日志，绝不影响主流程。
func generateSkillAsync(task string, transcript []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ 技能生成 panic: %v", r)
		}
	}()

	// 单工具、两步以内的任务没有沉淀价值
	if len(transcript) < 2 {
		return
	}

	prompt := fmt.Sprintf(`以下是一次成功完成的 Agent 编程任务的动作记录。
如果这个工作流对未来同类任务有复用价值，把它抽象成一个技能；如果只是一次性的琐碎操作，输出 {"name":""}。

任务：%s

动作序列：
%s

只输出一个 JSON 对象，不要任何解释和代码块包裹：
{"name":"kebab-case英文技能名","description":"一句话中文描述什么场景用这个技能","steps":["步骤1","步骤2"]}`,
		truncateChars(task, 500), strings.Join(transcript, "\n"))

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	content, _, err := routeChatOnce(ctx, resolveBackends("default", ""), msgs, nil)
	if err != nil {
		log.Printf("⚠️ 技能生成调用失败: %v", err)
		return
	}

	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var skill Skill
	if err := json.Unmarshal([]byte(content), &skill); err != nil {
		log.Printf("⚠️ 技能 JSON 解析失败: %v", err)
		return
	}
	skill.Name = skillNameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(skill.Name)), "-")
	skill.Name = strings.Trim(skill.Name, "-")
	if skill.Name == "" || len(skill.Steps) < 2 {
		return // 模型判定无复用价值
	}

	if err := os.MkdirAll(skillsDir(), 0755); err != nil {
		log.Printf("⚠️ 创建技能目录失败: %v", err)
		return
	}
	path := filepath.Join(skillsDir(), skill.Name+".json")
	data, _ := json.MarshalIndent(skill, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("⚠️ 写入技能文件失败: %v", err)
		return
	}
	log.Printf("🎓 新技能已沉淀: %s（%s）", skill.Name, skill.Description)
}
