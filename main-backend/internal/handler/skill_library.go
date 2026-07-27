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
//
// Source 在加载时按来源目录打标，不落盘（磁盘文件保持干净）。
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps,omitempty"`
	Body        string   `json:"body,omitempty"`   // 外部 SKILL.md 正文（markdown）
	Source      string   `json:"source,omitempty"` // learned | external
	// Status 只作用于自研技能：active 可按需加载，archived 保留在磁盘供恢复，
	// 不再使用候选/审阅状态，避免把治理成本推给用户。
	Status       string    `json:"status,omitempty"`
	Trigger      string    `json:"trigger,omitempty"`
	Verification string    `json:"verification,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	LastUsedAt   time.Time `json:"last_used_at,omitempty"`
	UseCount     int       `json:"use_count,omitempty"`
}

const (
	skillStatusActive   = "active"
	skillStatusArchived = "archived"
	skillActiveIdleTTL  = 45 * 24 * time.Hour
)

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

// loadLearnedSkills 扫描自研技能库目录，只返回 active 技能；闲置技能可恢复归档。
func loadLearnedSkills() []Skill {
	all := loadLearnedSkillsForSettings()
	skills := make([]Skill, 0, len(all))
	for _, s := range all {
		if s.Status == skillStatusActive {
			skills = append(skills, s)
		}
	}
	return skills
}

// loadLearnedSkillsForSettings 返回完整库存（启用/归档），供设置页治理使用。
// 生命周期清理仍在这里执行，但 archived 永不自动删除，保证可恢复。
func loadLearnedSkillsForSettings() []Skill {
	entries, err := os.ReadDir(skillsDir())
	if err != nil {
		return nil
	}
	var skills []Skill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(skillsDir(), e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Skill
		if json.Unmarshal(data, &s) != nil || s.Name == "" {
			continue
		}
		s.Source = "learned"
		info, err := e.Info()
		if err != nil {
			continue
		}
		changed := normalizeLearnedSkill(&s, info.ModTime())
		if s.Status == skillStatusActive && time.Since(skillLastActivity(s)) > skillActiveIdleTTL {
			s.Status = skillStatusArchived
			s.UpdatedAt = time.Now()
			changed = true
		}
		if changed {
			persistLearnedSkill(path, s)
		}
		skills = append(skills, s)
	}
	return skills
}

func normalizeLearnedSkill(s *Skill, fallback time.Time) bool {
	changed := false
	// 兼容旧格式：已有技能不在升级时消失，按文件修改时间开始计算闲置期。
	if s.Status == "" {
		s.Status = skillStatusActive
		changed = true
	}
	// 兼容之前短暂写入过的 candidate：不再要求用户审阅，直接恢复为可用。
	if s.Status == "candidate" {
		s.Status = skillStatusActive
		changed = true
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = fallback
		changed = true
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = fallback
		changed = true
	}
	return changed
}

func skillLastActivity(s Skill) time.Time {
	last := s.UpdatedAt
	if s.LastUsedAt.After(last) {
		last = s.LastUsedAt
	}
	if s.CreatedAt.After(last) {
		last = s.CreatedAt
	}
	return last
}

func persistLearnedSkill(path string, s Skill) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

func learnedSkillPath(name string) (string, error) {
	clean := skillNameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	clean = strings.Trim(clean, "-")
	if clean == "" || clean != name {
		return "", fmt.Errorf("技能名必须是 kebab-case 英文标识")
	}
	return filepath.Join(skillsDir(), clean+".json"), nil
}

// setLearnedSkillStatus 是设置页的显式治理入口。external SKILL.md 不经过这里，
// 因而绝不会被 GC 或 UI 的状态切换误伤。
func setLearnedSkillStatus(name, status string) (Skill, error) {
	if status != skillStatusActive && status != skillStatusArchived {
		return Skill{}, fmt.Errorf("不支持的技能状态")
	}
	path, err := learnedSkillPath(name)
	if err != nil {
		return Skill{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("技能不存在")
	}
	var s Skill
	if json.Unmarshal(data, &s) != nil || s.Name != name {
		return Skill{}, fmt.Errorf("技能文件无效")
	}
	s.Source, s.Status, s.UpdatedAt = "learned", status, time.Now()
	persistLearnedSkill(path, s)
	return s, nil
}

func deleteLearnedSkill(name string) error {
	path, err := learnedSkillPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("技能不存在")
		}
		return err
	}
	return nil
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
		skills = append(skills, Skill{Name: name, Description: desc, Body: body, Source: "external", Status: skillStatusActive})
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

const skillManageToolName = "skill_manage"

// skill_manage 仅供用户明确要求保存流程时使用；常规自动学习走后台质量门槛，
// 两者都直接启用，不设置人工审阅台阶。
var skillManageToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        skillManageToolName,
		Description: "创建并启用一个自研技能。仅在用户明确要求保存工作流经验时使用；技能必须是可复用、可验证的具体流程。",
		Parameters: core.ToolParameters{Type: "object", Properties: map[string]core.ToolProperty{
			"name":         {Type: "string", Description: "kebab-case 英文技能名"},
			"description":  {Type: "string", Description: "不超过 60 字的中文用途描述"},
			"trigger":      {Type: "string", Description: "明确的使用条件"},
			"verification": {Type: "string", Description: "可执行或可观察的成功验证"},
			"steps":        {Type: "array", Description: "3 到 6 条具体、可执行步骤", Items: &core.ToolProperty{Type: "string"}},
		}, Required: []string{"name", "description", "trigger", "verification", "steps"}},
	},
}

func handleSkillManage(argsJSON string) string {
	var skill Skill
	if err := json.Unmarshal([]byte(argsJSON), &skill); err != nil {
		return "参数解析失败：需要 name、description、trigger、verification、steps"
	}
	skill.Name = skillNameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(skill.Name)), "-")
	skill.Name = strings.Trim(skill.Name, "-")
	skill.Description, skill.Trigger, skill.Verification = strings.TrimSpace(skill.Description), strings.TrimSpace(skill.Trigger), strings.TrimSpace(skill.Verification)
	if skill.Name == "" || skill.Description == "" || skill.Trigger == "" || skill.Verification == "" || len(skill.Steps) < 3 || len(skill.Steps) > 6 {
		return "技能未保存：请给出 kebab-case 名称、描述、触发条件、验证方式和 3–6 个步骤"
	}
	if err := os.MkdirAll(skillsDir(), 0755); err != nil {
		return "技能未保存：" + err.Error()
	}
	path := filepath.Join(skillsDir(), skill.Name+".json")
	if _, err := os.Stat(path); err == nil {
		return "技能未保存：同名技能已存在，请在设置页审阅或换名"
	}
	skill.Source, skill.Status = "learned", skillStatusActive
	skill.CreatedAt, skill.UpdatedAt = time.Now(), time.Now()
	persistLearnedSkill(path, skill)
	return fmt.Sprintf("技能已保存并启用：%s。", skill.Name)
}

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
		if s.Source == "learned" {
			markSkillUsed(s)
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

func markSkillUsed(s Skill) {
	if s.Source != "learned" || s.Name == "" {
		return
	}
	path := filepath.Join(skillsDir(), s.Name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var disk Skill
	if json.Unmarshal(data, &disk) != nil || disk.Name != s.Name {
		return
	}
	disk.UseCount++
	disk.LastUsedAt = time.Now()
	disk.UpdatedAt = disk.LastUsedAt
	if disk.Status == "" {
		disk.Status = skillStatusActive
	}
	persistLearnedSkill(path, disk)
}

var skillNameSanitizer = regexp.MustCompile(`[^a-z0-9\-]+`)

// generateSkillAsync 在工作流成功后异步抽象技能。失败只打日志，绝不影响主流程。
func generateSkillAsync(task string, transcript []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ 技能生成 panic: %v", r)
		}
	}()

	// Hermes 风格的低摩擦学习：只在真正复杂的已完成任务后后台尝试提炼，
	// 由模型再判断是否值得保存；通过质量门槛的产物直接启用，不打扰用户。
	if len(transcript) < 5 {
		return
	}

	prompt := fmt.Sprintf(`以下是一次成功完成的 Agent 编程任务的动作记录。
如果这个工作流对未来同类任务有复用价值，把它抽象成一个技能；如果只是一次性的琐碎操作，输出 {"name":""}。

任务：%s

动作序列：
%s

只输出一个 JSON 对象，不要任何解释和代码块包裹。技能必须有明确触发条件、可验证结果，且只保留 3–6 个可执行步骤；不得写入临时路径、密钥或项目专属垃圾信息：
{"name":"kebab-case英文技能名","description":"一句话中文描述什么场景用这个技能","trigger":"何时调用","verification":"如何验证成功","steps":["步骤1","步骤2","步骤3"]}`,
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
	if skill.Name == "" || len(skill.Steps) < 3 || len(skill.Steps) > 6 || skill.Trigger == "" || skill.Verification == "" {
		return // 模型判定无复用价值
	}

	if err := os.MkdirAll(skillsDir(), 0755); err != nil {
		log.Printf("⚠️ 创建技能目录失败: %v", err)
		return
	}
	// 自动学习不制造待办：通过质量门槛后直接启用；后台 curator 负责后续归档。
	skill.Status = skillStatusActive
	skill.CreatedAt = time.Now()
	skill.UpdatedAt = skill.CreatedAt
	path := filepath.Join(skillsDir(), skill.Name+".json")
	data, _ := json.MarshalIndent(skill, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("⚠️ 写入技能文件失败: %v", err)
		return
	}
	log.Printf("🎓 新技能已自动启用: %s（%s）", skill.Name, skill.Description)
}
