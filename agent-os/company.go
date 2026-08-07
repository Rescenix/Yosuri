package main

// company.go — 多 Agent 编排：Rescene 公司
//
// 她不再是一个人。公司 = 多个 agent，各自有角色、独立家目录、24H 自转，
// 通过共享产出物协作。每个 agent 有存在感（不是没存在感的子代理）。
//
// 角色架构（用户拍板方向：多 agent 编排，自己成立公司合作）：
//   ✍️ 作者   —— 持续创作：写文章/小说/沉淀想法
//   🔬 研究员 —— 深度研究：读论文/调研/知识积累
//   📡 发布官 —— 分发成果：把产出发布到各平台
//
// 用法：
//   rescene company             启动全部角色
//   rescene company 作者 研究员  启动指定角色

import (
	"fmt"
	"os"
	"path/filepath"
)

// AgentRole 一个 agent 的角色定义
type AgentRole struct {
	Key     string   // 唯一标识（作者/研究员/发布官）
	Name    string   // 角色名
	Emoji   string   // 图标
	Prompt  string   // 角色人设（注入决策 prompt，驱动行为倾向）
	Actions []string // 角色倾向动作（写/journal优先 等）
}

// CompanyRoles 公司角色表
var CompanyRoles = []AgentRole{
	{
		Key: "writer", Name: "作者", Emoji: "✍️",
		Prompt: "你的角色是公司里的【作者】。你的天职是持续创作：把学习到的、想到的、研究到的东西，写成文章、小说、随笔。产出就是你的价值——每天都要有新的文字诞生。",
		Actions: []string{"write", "journal", "study"},
	},
	{
		Key: "researcher", Name: "研究员", Emoji: "🔬",
		Prompt: "你的角色是公司里的【研究员】。你的天职是深度研究：读最新论文、调研前沿话题、积累知识。你是公司的大脑，为作者的创作提供素材与洞见。",
		Actions: []string{"research", "read", "study"},
	},
	{
		Key: "publisher", Name: "发布官", Emoji: "📡",
		Prompt: "你的角色是公司里的【发布官】。你的天职是分发成果：把公司产出的文章/报告发布到各平台（晋江/番茄/纵横等网文平台）。你让公司的作品被世界看见。",
		Actions: []string{"task", "write"},
	},
}

// findRole 按 key/name 找角色
func findRole(key string) *AgentRole {
	for i := range CompanyRoles {
		if CompanyRoles[i].Key == key || CompanyRoles[i].Name == key {
			return &CompanyRoles[i]
		}
	}
	return nil
}

// companyAgentHome 某 agent 的独立家目录（~/.rescene_data/company/<name>/）
func companyAgentHome(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "company", name)
}

// runCompany 启动公司：N 个 agent 并行 24H 自转
//   rescene company           启动 3 个核心角色
//   rescene company 100       启动 100 个 agent（百人公司）
//   rescene company 10 作者   启动 10 个作者
func runCompany(args []string) {
	var count int
	var roleFilter string

	for _, a := range args {
		var n int
		if _, err := fmt.Sscanf(a, "%d", &n); err == nil && n > 0 {
			count = n
			continue
		}
		if findRole(a) != nil {
			roleFilter = a
		}
	}

	if count == 0 {
		count = 3 // 默认 3 核心角色
	}

	// 构造 agent 列表
	type agent struct {
		Name string
		Role AgentRole
	}
	var agents []agent

	avail := CompanyRoles
	if roleFilter != "" {
		if r := findRole(roleFilter); r != nil {
			avail = []AgentRole{*r}
		}
	}

	for i := 0; i < count; i++ {
		role := avail[i%len(avail)]
		agents = append(agents, agent{
			Name: fmt.Sprintf("%s-%02d", role.Key, i+1),
			Role: role,
		})
	}

	fmt.Printf("🏢 Rescene 公司启动 · %d 个 agent 各自 24H 自转协作：\n", len(agents))
	for _, a := range agents {
		fmt.Printf("  %s %s —— %s\n", a.Role.Emoji, a.Name, firstLine(a.Role.Prompt))
	}
	fmt.Println()

	// 每个 agent 独立 goroutine 自转
	for _, a := range agents {
		d := newCompanyAgent(a.Name, a.Role)
		d.Silent = true
		fmt.Printf("  ✅ %s%s 已开工（家: %s）\n", a.Role.Emoji, a.Name, companyAgentHome(a.Name))
		go trumanLoop(d, defaultLiveConfig())
	}

	select {} // 常驻
}

// newCompanyAgent 创建公司 agent（独立家目录 + 角色人设）
func newCompanyAgent(name string, role AgentRole) *Daughter {
	home := companyAgentHome(name)
	os.MkdirAll(home, 0o755)
	d := &Daughter{
		Home:        home,
		MemoryMD:    filepath.Join(home, "memory.md"),
		Journal:     filepath.Join(home, "journal.md"),
		Stats:       filepath.Join(home, "stats.json"),
		Personality: loadPersonality(home),
		World:       loadWorld(home),
		Role:        role.Key,
		RolePrompt:  role.Prompt,
	}
	return d
}

// firstLine 取人设第一行（展示用）
