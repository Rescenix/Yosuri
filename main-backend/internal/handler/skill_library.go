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

	"backend/internal/ai/core"
)

// Skill 统一承载两类技能：
//   - 自研沉淀（Source=learned）：工作流成功后抽象出的 JSON，正文在 Steps。
//   - 外部导入（Source=external）：Anthropic/Claude 风格的 SKILL.md，正文在 Body。
// Source 在加载时按来源目录打标，不落盘（磁盘文件保持干净）。
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps,omitempty"`
	Body        string   `json:"body,omitempty"`   // 外部 SKILL.md 正文（markdown）
	Source      string   `json:"source,omitempty"` // learned | external
}

func skillsDir() string {
	if dir := os.Getenv("AURORA_SKILLS_DIR"); dir != "" {
		return dir
	}
	return "./skills"
}

// externalSkillsDir 是外部技能的挂载点：往这里丢 Anthropic/Claude 风格的 SKILL.md
// 文件夹即可被 agent 加载，与自研沉淀的 ./skills 互不干扰。
func externalSkillsDir() string {
	if dir := os.Getenv("AURORA_EXT_SKILLS_DIR"); dir != "" {
		return dir
	}
	return "./skills-ext"
}

// loadSkills 返回全部可用技能：自研沉淀 + 外部导入。
// skillLibraryPrompt（索引）和 handleReadSkill（取全文）共用这一份数据源，
// 就像 mcpToolIndexPrompt 和 handleLoadTools 共用 loadMCPToolDefs 一样。
func loadSkills() []Skill {
	return append(loadLearnedSkills(), loadExternalSkills()...)
}

// loadLearnedSkills 扫描自研技能库目录（./skills/*.json），打 Source=learned。
func loadLearnedSkills() []Skill {
	entries, err := os.ReadDir(skillsDir())
	if err != nil {
		return nil
	}
	var skills []Skill
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
		s.Source = "learned"
		skills = append(skills, s)
	}
	return skills
}

// loadExternalSkills 扫描外部技能目录：每个子目录一个 SKILL.md（Anthropic/Claude 格式），
// 也兼容平铺的 *.md 文件。frontmatter 取 name/description，围栏后的正文进 Body。
func loadExternalSkills() []Skill {
	root := externalSkillsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var skills []Skill
	add := func(path, fallbackName string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		name, desc, body := parseSkillMD(string(data))
		if name == "" {
			name = fallbackName
		}
		if name == "" {
			return
		}
		skills = append(skills, Skill{Name: name, Description: desc, Body: body, Source: "external"})
	}
	for _, e := range entries {
		if e.IsDir() {
			add(filepath.Join(root, e.Name(), "SKILL.md"), e.Name())
		} else if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			add(filepath.Join(root, e.Name()), strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		}
	}
	return skills
}

// parseSkillMD 解析 Anthropic/Claude 风格的 SKILL.md：--- 围栏内的 YAML frontmatter
// 取 name/description，围栏之后为正文。只认这两个字段，不引 YAML 依赖（够用即可）。
func parseSkillMD(content string) (name, desc, body string) {
	s := strings.ReplaceAll(content, "\r\n", "\n")
	if strings.HasPrefix(s, "---\n") {
		if end := strings.Index(s[4:], "\n---"); end >= 0 {
			fm := s[4 : 4+end]
			body = strings.TrimLeft(s[4+end+4:], "\n")
			for _, line := range strings.Split(fm, "\n") {
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				k = strings.TrimSpace(k)
				v = strings.Trim(strings.TrimSpace(v), `"'`)
				switch k {
				case "name":
					name = v
				case "description":
					desc = v
				}
			}
			return name, desc, body
		}
	}
	return "", "", s // 没有 frontmatter：整篇当正文，名字由调用方兜底
}

// skillLibraryPrompt 把技能库整理成系统提示词片段；库为空时返回空串。
// 只注入名称+描述，正文步骤不进上下文（token 是成本）——需要完整步骤时
// 模型调 read_skill 按名字取（见 handleReadSkill），不再像过去那样永远拿不到。
func skillLibraryPrompt() string {
	skills := loadSkills()
	if len(skills) == 0 {
		return ""
	}
	lines := make([]string, 0, len(skills))
	for _, s := range skills {
		tag := ""
		if s.Source == "external" {
			tag = "[外部] " // 官方/外部导入的技能，正文是说明文档而非步骤
		}
		lines = append(lines, fmt.Sprintf("- %s%s：%s", tag, s.Name, s.Description))
	}
	sort.Strings(lines)
	return "\n━━━ 技能库索引（按需加载，用 read_skill 取完整内容） ━━━\n" + strings.Join(lines, "\n") + "\n"
}

// readSkillToolName 是取回技能完整步骤的钥匙，跟 load_tools 一样必须常驻工具集。
const readSkillToolName = "read_skill"

var readSkillToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: readSkillToolName,
		Description: "按名字取回技能库里某个技能的完整内容。系统提示词里的「技能库索引」" +
			"只给了名字和一句话描述，要看具体怎么做，先用这个取回完整内容（可一次传多个）：" +
			"自研技能给 steps 步骤，[外部] 技能给 content 说明文档。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"names": {
					Type:        "array",
					Description: "要取回的技能名数组，必须与索引里的名字完全一致",
					Items:       &core.ToolProperty{Type: "string"},
				},
			},
			Required: []string{"names"},
		},
	},
}

// handleReadSkill 处理一次 read_skill 调用：按名字查找技能库，把完整
// {name, description, steps} 作为工具结果回给模型。纯查询，没有 load_tools
// 那样的"激活"副作用，不影响 tools 数组。
//
// 不存在的名字不是致命错误——回一句"没有这个技能"，让模型对着索引改。
func handleReadSkill(argsJSON string, skills []Skill) string {
	var args struct {
		Names []string `json:"names"`
		// 容错：模型有时会传单个字符串而不是数组
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "参数解析失败，names 应为字符串数组，例如 {\"names\":[\"deploy-frontend\"]}"
	}
	names := args.Names
	if len(names) == 0 && args.Name != "" {
		names = []string{args.Name}
	}
	if len(names) == 0 {
		return "names 为空，请指定要取回的技能名（见系统提示词里的技能库索引）"
	}

	byName := map[string]Skill{}
	for _, s := range skills {
		byName[s.Name] = s
	}

	var found []map[string]any
	var missing []string
	for _, n := range names {
		s, ok := byName[n]
		if !ok {
			missing = append(missing, n)
			continue
		}
		entry := map[string]any{
			"name": s.Name, "description": s.Description, "source": s.Source,
		}
		// 自研技能给步骤，外部技能给正文文档——两类只会有其一
		if len(s.Steps) > 0 {
			entry["steps"] = s.Steps
		}
		if s.Body != "" {
			entry["content"] = s.Body
		}
		found = append(found, entry)
	}

	var b strings.Builder
	if len(found) > 0 {
		schemas, _ := json.MarshalIndent(found, "", "  ")
		fmt.Fprintf(&b, "已取回 %d 个技能的完整步骤：\n%s", len(found), schemas)
	}
	if len(missing) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "以下技能名在技能库索引里不存在：%s\n请对照系统提示词里的索引核对名字。",
			strings.Join(missing, "、"))
	}
	return b.String()
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
