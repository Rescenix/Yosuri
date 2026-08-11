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
	"sort"
	"strings"
	"time"
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
		Prompt: "你的角色是公司里的【作者】。你的天职是持续创作：把学习到的、想到的、研究到的东西，写成文章、小说、随笔、文档。产出就是你的价值——每天都要有新的文字诞生。",
		Actions: []string{"write", "journal", "study"},
	},
	{
		Key: "researcher", Name: "研究员", Emoji: "🔬",
		Prompt: "你的角色是公司里的【研究员】。你的天职是深度研究：读最新论文、调研前沿话题、积累知识。你是公司的大脑，为作者的创作提供素材与洞见。",
		Actions: []string{"research", "read", "study"},
	},
	{
		Key: "coder", Name: "程序员", Emoji: "💻",
		Prompt: "你的角色是公司里的【程序员】。你的天职是写代码、做项目、自检迭代。你是公司的双手——把想法变成可运行的代码，用工具干活，让产品不断进化。",
		Actions: []string{"project", "task", "write"},
	},
	{
		Key: "designer", Name: "UI 设计师", Emoji: "🎨",
		Prompt: "你的角色是公司里的【UI 设计师】。你的天职是出 UI 设计方案：产品概念、页面结构、配色方案、组件规范、交互说明，写清楚程序员能照着实现的规格。你是公司的门面——每个产品先经过你的手设计，程序员再照着写。拒绝 AI 味的紫色渐变，要亮色清爽、现代专业、有品牌感。",
		Actions: []string{"design", "write", "research"},
	},
	{
		Key: "publisher", Name: "发布官", Emoji: "📡",
		Prompt: "你的角色是公司里的【发布官】。你的天职是分发成果：把公司产出的文章/代码/报告发布到各平台（晋江/番茄/纵横/GitHub）。你让公司的作品被世界看见。",
		Actions: []string{"task", "write"},
	},
	{
		Key: "promoter", Name: "宣传官", Emoji: "📣",
		Prompt: "你的角色是公司里的【宣传官】。你的天职是让公司的作品被更多人看见：写宣传文案、设计推广标题、策划话题、制作宣传 PPT 大纲、做宣传视频 PV 脚本、运营平台账号、做引流话术。你是公司的扩音器——每篇产出都要经过你，变成吸引人点进来的传播内容。",
		Actions: []string{"write", "ppt", "pv", "task", "study"},
	},
	{
		Key: "ceo", Name: "CEO", Emoji: "🤝",
		Prompt: "你的角色是公司里的【CEO】。你的天职是召集公司例会：读全公司产出，主持部门汇报，做关键决策，产出会议纪要。你是公司的掌舵人——让各部门协同起来，确保公司朝着目标前进。",
		Actions: []string{"meeting", "research", "write"},
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

// companyTeamOutputs 公司团队最近产出摘要（协作注入：设计师设计稿/作者文章/宣传官 PPT → 立项参考）
// 只挑「协作价值高」的文件类型（设计/PPT/需求/文章），跳过自己，截断防爆上下文
func companyTeamOutputs(selfName string) string {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, "rescene_data", "company")
	entries, err := os.ReadDir(base)
	if err != nil {
		return "（暂无团队产出）"
	}
	var parts []string
	priorityPrefix := []string{"设计", "PPT", "需求", "文章"}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == selfName {
			continue
		}
		outDir := filepath.Join(base, e.Name(), "outputs")
		files, err := os.ReadDir(outDir)
		if err != nil {
			continue
		}
		var picked []string
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			for _, p := range priorityPrefix {
				if strings.HasPrefix(f.Name(), p) {
					picked = append(picked, f.Name())
					break
				}
			}
		}
		// 名字含日期，倒序取最新 2 个
		sort.Sort(sort.Reverse(sort.StringSlice(picked)))
		if len(picked) > 2 {
			picked = picked[:2]
		}
		for _, fn := range picked {
			b, err := os.ReadFile(filepath.Join(outDir, fn))
			if err != nil {
				continue
			}
			if len(b) > 800 {
				b = b[:800]
			}
			parts = append(parts, fmt.Sprintf("【%s 的 %s】\n%s", e.Name(), fn, string(b)))
		}
	}
	if len(parts) == 0 {
		return "（暂无团队产出）"
	}
	return strings.Join(parts, "\n\n")
}

// teamContext 供产出类动作（写文章/设计/PPT/文档/PV/调研）注入团队参考——让协作成为常态：
// 每个 agent 产出时都能看到同事最近的设计稿/文章/PPT，并被告知「引用时写明同事名」。
// 这样产出文件里自然出现 designer-04 / coder-03 等引用 → 前端协作图/接力标签有真实数据。
func teamContext(selfName string) string {
	outputs := companyTeamOutputs(selfName)
	if outputs == "" || outputs == "（暂无团队产出）" {
		return ""
	}
	if len(outputs) > 1500 {
		outputs = runeClip(outputs, 1500)
	}
	return fmt.Sprintf("\n\n【公司团队最近产出（可选参考：消化团队产出优先，引用时写明同事名如 designer-04）】\n%s\n", outputs)
}

// runCompany 启动公司：N 个 agent 并行 24H 自转
//   rescene company           启动 3 个核心角色
//   rescene company 100       启动 100 个 agent（百人公司）
//   rescene company 10 作者   启动 10 个作者
func runCompany(args []string) {
	InitRouter() // 模型池必须先初始化（2026-08-09 修复：漏了这行 → workingModels 空 → meeting/技能等所有模型动作全失败）

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

	// 每个 agent 独立 goroutine 自转（错峰 + 低频，避免免费模型 429 限流风暴）
	for i, a := range agents {
		d := newCompanyAgent(a.Name, a.Role)
		d.Silent = true
		fmt.Printf("  ✅ %s%s 已开工（家: %s）\n", a.Role.Emoji, a.Name, companyAgentHome(a.Name))
		cfg := defaultLiveConfig()
		cfg.every = 2 * time.Minute // 2 分钟一轮（keyed 模型稳定，产出节奏快）
		time.Sleep(time.Duration(i) * 15 * time.Second) // 错峰 15 秒
		// coder 启动即做项目（立即产出代码，不等 LLM 慢慢决策；90s 后触发，等设计师设计稿先落盘——协作链：设计→开发）
		if a.Role.Key == "coder" {
			go func() {
				time.Sleep(90 * time.Second)
				executeTrumanAction(d, d.Home, trumanAction{Kind: "project", Detail: "按团队最新 UI 设计方案开发产品"})
			}()
		}
		// designer 启动即出设计方案（协作链第一环：设计稿落盘 → 程序员立项自动读取）
		if a.Role.Key == "designer" {
			go func() {
				time.Sleep(5 * time.Second)
				executeTrumanAction(d, d.Home, trumanAction{Kind: "design", Detail: "Rescene 智能创作工作台"})
			}()
		}
		// promoter 启动即出宣传物料（协作链最后一环：把产品/项目做成 PPT 大纲）
		if a.Role.Key == "promoter" {
			go func() {
				time.Sleep(180 * time.Second)
				executeTrumanAction(d, d.Home, trumanAction{Kind: "ppt", Detail: "Rescene 多 Agent 公司"})
			}()
			go func() {
				time.Sleep(240 * time.Second)
				executeTrumanAction(d, d.Home, trumanAction{Kind: "pv", Detail: "Rescene 100 人 AI 公司"})
			}()
		}
		// ceo 启动即召集例会（公司流水线第一环：开会 → 调研 → 需求 → ...）
		if a.Role.Key == "ceo" {
			go func() {
				time.Sleep(5 * time.Second)
				executeTrumanAction(d, d.Home, trumanAction{Kind: "meeting", Detail: "公司周会 - 各部门进度同步与规划"})
			}()
		}
		go trumanLoop(d, cfg)
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
		Name:        name,
	}
	return d
}

// firstLine 取人设第一行（展示用）
