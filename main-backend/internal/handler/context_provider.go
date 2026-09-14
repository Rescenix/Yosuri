package handler

// ContextProvider —— 上下文装配的唯一入口。
//
// 解决三个真实存在的问题（不是为了抽象而抽象）：
//
//  1. 顺序是随手拼出来的。原来系统提示词由一串 `systemPrompt += ...` 攒成，
//     谁先谁后取决于代码行的先后。而前缀缓存只认「从头开始逐字节相同」——
//     把每次任务后都会变的技能库排在稳定内容前面，等于每完成一个任务就把
//     后面所有内容的缓存全作废。这里显式声明每段的稳定性，装配时稳定段一律靠前。
//
//  2. 段落被写了两遍，会漂。原来 `+=` 链拼一遍、contextBreakdown 里再列一遍，
//     加一段就得记得改两处——加 MCP 工具索引时就漏过一次（前端分类少算了 746 tok）。
//     现在 sections 是唯一事实来源，提示词和分类占用都由它派生。
//
//  3. 工具激活状态没有归属。按需加载（tool_ondemand.go）的 activated 集合原来
//     裸挂在 handler 的局部变量里，主 Agent 和子代理各拼各的工具数组。
//
// 命名沿用调用生命周期：Invoking 装配这一轮要发出去的东西，Invoked 落这一轮的状态。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/agent"
	"backend/internal/ai/core"
	"backend/internal/knowledge"
	"backend/internal/memorydir"
)

// contextSection 系统提示词里的一段。
// stable=true 表示"进程生命周期内基本不变"，装配时排在前面以利前缀缓存。
type contextSection struct {
	key     string // 前端 context 面板的分类名，也是 breakdown 的 key
	content string
	stable  bool
}

// contextProvider 一次工作流的上下文装配器。非并发安全——一个工作流一个实例，
// 全程在同一个 goroutine 里用（子代理有自己的实例）。
type contextProvider struct {
	sections []contextSection
	// rp 角色扮演模式：不暴露任何工具（模型只写戏，不碰文件/命令/网络），
	// 系统提示词换成 RP 导演契约 + 角色卡人设，技能库/工具索引一律不注入。
	rp bool
	// activated 已被 load_tools 激活的 Go 内置/MCP 工具，决定 Tools() 返回什么
	activated map[string]bool
	// hotSkillName 本轮被热集管线预加载全文的技能名（空=无）。工作流成功收尾时
	// 据此补记一次使用——预加载后模型不会再调 skill_view，不补记的话分数自然衰减
	// 掉出热集，下次冷启动白费一轮 skill_view（热者恒热的正确姿势是"真的还在用"）。
	hotSkillName string
	// onInvoked 每轮收尾时的落状态回调（当前用于落检查点）。
	// 不由 provider 自己存盘：轮次内的 msgs/token 统计属于循环，provider 不该假装拥有它们。
	onInvoked func(round int, st roundState)
}

// roundState 一轮结束时需要落盘的可变状态。provider 不持有它们，只负责在
// Invoked 时把它们交给落状态回调——谁产生谁拥有，避免 provider 变成上帝对象。
type roundState struct {
	msgs         []map[string]any
	transcript   []string
	callSigCount map[string]int
	callSeq      int
	inputTokens  int
	outputTokens int
	// todos agent 自己维护的任务清单。进检查点是为了续跑不丢主线——
	// 计划原本只存在于可能被压缩折叠掉的 tool_calls 参数里。
	todos []todoItem
}

// newWorkflowContextProvider 组装主 Agent 的上下文。
// 段落顺序即装配顺序，stable 的排前面——注意这不是随意排的：
// system/subagent/工具索引在进程内基本不动，skill 每完成一个任务就可能变，
// memory 每写一次记忆就变，把后两者放在最后，前面那一大段的缓存才活得下来。
func newWorkflowContextProvider(tasks ...string) *contextProvider {
	return newWorkflowContextProviderFor("", tasks...)
}

// newWorkflowContextProviderFor 按 agent 装配上下文。agentID 为空时行为与
// 改造前完全一致（只有通用记忆）——单 Agent 的老链路零影响。
//
// agentID 非空时额外挂上该 Agent 的私有记忆：
//   - 私有记忆索引（无条件注入，等价通用 index）
//   - 私有记忆联想召回（按任务 bigram 命中，与通用召回同一预算策略）
//
// 通用记忆（~/rescene_data/memory/）照旧注入：所有 Agent 共享同一份用户画像、
// 偏好、项目笔记；私有记忆是「这个 Agent 自己经历出来的东西」。
func newWorkflowContextProviderFor(agentID string, tasks ...string) *contextProvider {
	memorySection := ""
	inject := memorydir.ReadIndex()
	if inject != "" {
		memorySection = "\n\n# 长期记忆索引\n" + inject
	}
	workdirSection := projectWorkdirPrompt()
	// 常驻记忆 pinned.md：memory_pin 写入，每轮无条件注入（等价于身份常驻）。
	pinnedSection := ""
	if pinned := memorydir.ReadPinned(); pinned != "" {
		pinnedSection = "\n\n# 常驻记忆\n" + pinned
	}
	// 会话交接 handoff.md：memory_handoff 写入，跨对话不失业。
	handoffSection := ""
	if handoff := memorydir.ReadHandoff(); handoff != "" {
		handoffSection = "\n\n# 会话交接（上次留下的工作态）\n" + handoff
	}
	task := ""
	if len(tasks) > 0 {
		task = tasks[0]
	}

	// ── 亲密等级（外显等级，无上限）：驱动记忆机制 ──
	// 云端权威（随 UID 账号存 ResceneCloud），本地缓存 memory/intimacy.md 供每轮注入：
	//   - 注入亲密等级 → 模型感知与用户的关系亲疏，语气自然调整（越熟越自然）
	//   - Lv≥2（100 互动）：偏好自动回填 —— 无条件注入 preferences.md，不用等 bigram 命中
	//   - Lv≥5（1000 互动）：关联记忆召回加深 —— 命中文件数 3 → 5（更深的记忆展开）
	// 等级换算用 QQ 宠物式曲线（越高越难升），本身无上限，能一直升。
	_, intimacyVal := memorydir.ReadIntimacy()
	level := memorydir.IntimacyLevel(intimacyVal)

	intimacySection := ""
	if intimacyVal > 0 {
		intimacySection = fmt.Sprintf("\n\n# 亲密等级（与用户的关系等级，无上限）\n你和当前用户的亲密等级是 Lv.%d。\n等级反映你们相处的时间与互动积累：等级越高代表你们越熟、越有默契（升级会越来越慢，但永不封顶）。\n- 低等级：保持礼貌、简洁、专业。\n- 高等级：可以更自然、亲切、体贴，像熟悉的朋友一样主动分享想法。\n自然地融入语气即可，不要刻意提及等级数字。", level)
	}
	// 性格档案：从记忆蒸馏的语气/长度/忌讳，千人千面。
	// 归 system 桶（同属"给模型的指令"），排在亲密度后面。
	personalitySection := memorydir.ReadRaw("personality")
	if strings.TrimSpace(personalitySection) != "" {
		personalitySection = "\n\n# 性格档案（从你的记忆蒸馏，千人千面）\n" + strings.TrimSpace(personalitySection) + "\n"
	}
	// 用户偏好：无条件注入（2026-09-08 用户拍板：跟 Hermes 画像同待遇，不设亲密度门槛）。
		// 之前 Lv≥2 才回填——新用户前 100 互动看不到偏好，等于画像白攒。现在每轮常驻。
		prefSection := ""
		if pref := memorydir.ReadRaw("preferences"); pref != "" {
			prefSection = "\n\n# 用户偏好（自动提取，常驻）\n" + pref
		}

	// 反向链接联想召回：根据当前任务匹配 index.md 中的行，
	// 命中的 [[文件]] 自动读取对应文件内容（亲密等级越高召回越深）
	taskMemory := ""
	if task != "" {
		maxFiles := 3
		if level >= 5 {
			maxFiles = 5
		}
		if linked := memorydir.ReadWithLinksLimit(task, maxFiles); linked != "" && linked != inject {
			taskMemory = "\n\n# 关联记忆（按任务联想读取）\n" + linked
		}
	}

	// 外挂知识库 RAG：用户丢进 ~/rescene_data/knowledge/ 的文档（md/txt/docx/pptx/pdf），
	// 按任务检索召回相关片段。与 memory 互补：memory 是 agent 沉淀的精简记忆（每轮注入），
	// knowledge 是用户提供的大体量文档（只按需召回相关段，控制 token）。
	knowledgeSection := ""
	if task != "" {
		if hit := knowledge.Search(task, 3); hit != "" {
			knowledgeSection = "\n\n# 外挂知识库（检索召回的相关片段，仅作参考，以你本地文件/实际代码为准）\n" + hit
		}
	}

	// ── 该 Agent 的私有记忆（agentID 为空则整段跳过）──
	// AgentReadWithLinks 的返回值本身已带私有索引，所以命中时只注入召回段，
	// 未命中（空串）才退回只注入索引，避免同一份索引出现两遍。
	agentID = memorydir.SanitizeAgentID(agentID)
	agentMemorySection := ""
	if agentID != "" {
		maxFiles := 3
		if level >= 5 {
			maxFiles = 5
		}
		recall := ""
		if task != "" {
			recall = memorydir.AgentReadWithLinks(agentID, task, maxFiles)
		}
		if strings.TrimSpace(recall) != "" {
			agentMemorySection = "\n\n# 我的私有记忆（只属于我这个 Agent，按任务联想读取）\n" + strings.TrimSpace(recall)
		} else if m := memorydir.AgentReadIndex(agentID); m != "" {
			agentMemorySection = "\n\n# 我的私有记忆索引（只属于我这个 Agent）\n" + m
		}
	}

	// 热门技能动态管线：按真实使用频率（账本衰减分）选 top-1 预加载全文。
	// 冷启动账本为空 → 热集为空 → 退化为纯索引模式。命名记在 provider 上，
	// 供工作流收尾补记一次使用（防"预加载后模型不再 skill_view → 分数掉出 → 死循环"）。
	hotSection, hotState := hotSkillsPrompt(loadSkills())

	return &contextProvider{
		activated:    map[string]bool{},
		hotSkillName: hotState.Name,
		sections: []contextSection{
			// —— 稳定段：进程内基本不变，构成前缀缓存的主体 ——
			{key: "system", content: agent.MainAgentConfigNative().SystemPrompt, stable: true},
			// 历史任务的读法必须由系统明说，不能指望模型自己从格式里悟。
			// 归到 system 桶，几十个 token，但直接决定它会不会回头重做旧任务。
			{key: "system", content: historyContractPrompt, stable: true},
			{key: "subagent", content: subAgentUsagePrompt, stable: true},
			// 工具索引只在内置工具或 MCP server 增删时变（很少），算稳定段；
			// 完整 schema 靠 load_tools 按需取，见 tool_ondemand.go。
			// key 用 "tools" 是有意的：前端 contextBreakdown.js 只认
			// system/subagent/skill/memory/tools 五个桶，索引归到工具桶里，
			// 免得凭空多一个前端会丢掉的 key，害「分类之和 ≈ prompt_tokens」对不上。
			{key: "tools", content: mcpToolIndexPrompt() + nativeToolIndexPrompt(), stable: true},
			// 技能库索引：只注入名称+描述，正文一律由模型自己调 skill_view 取回，
			// 宿主不做全文预加载（token 是成本，且索引段已强制要求"命中必须先取全文再动手"）。
			// 进程内极少变，放稳定段保证每轮都在。
			{key: "skill", content: skillLibraryPrompt(), stable: true},

			// —— 易变段：一变就让它后面的缓存作废，所以一律排在最后 ——
			// 热门技能全文预加载（频率驱动，非关键词猜测）。热集稳定时这段内容不变，
			// 换人才作废它后面的 memory 缓存——滞回设计就是为减少这种翻转。
			{key: "skill", content: hotSection},
			// memory 主体：预算内注入"最重要的记忆"（常驻/亲密/偏好/项目/工作态/联想），
			// 超出预算的低优先块直接丢弃；index 单独作尾部索引，始终完整保留供反向链接展开。
			{key: "memory", content: combineMemoryWithBudget([]memPart{
				{0, pinnedSection}, {1, intimacySection}, {2, prefSection},
				{3, workdirSection}, {4, handoffSection}, {5, taskMemory},
			})},
			{key: "memory", content: memorySection},    // 尾部记忆索引（agent 按需反向链接展开）
			{key: "memory", content: knowledgeSection}, // 外挂知识库 RAG：检索召回相关片段
			// 私有记忆：只属于当前 Agent 的经历（agentID 为空时是空串，不占 token）
			{key: "memory", content: agentMemorySection},
			// 自定义指令归到 system 桶（同属"给模型的指令"，且只有十几 tok，
			// 单开一个桶不值得改前端契约）。原来它根本没进 breakdown，是个漏登记。
			{key: "system", content: userInstructionsPrompt()},
			// 性格档案归 system 桶（从记忆蒸馏，千人千面），优先级最低（可被 persona 覆盖）。
			{key: "system", content: personalitySection},
		},
	}
}

// WithPersona 注入前端传来的「人设」段（/api/code/workflow 的 persona 参数）。
// 空串则忽略（保持纯中性基底）。人设排在整个系统提示词第一段、归稳定段，
// 与 MainAgentConfigNative 的中性基底互补：性格/称呼/语气全由前端预设驱动，
// 后端不再写死任何自称（原「你是 Rescene酱…」已移为前端默认预设）。
func (p *contextProvider) WithPersona(persona string) *contextProvider {
	persona = strings.TrimSpace(persona)
	if persona == "" {
		return p
	}
	p.sections = append([]contextSection{{key: "system", content: persona + "\n\n", stable: true}}, p.sections...)
	return p
}

// parseCastParam 解析 RP cast 参数（逗号分隔的角色卡 id），过滤非法 id，
// 空结果回退到单个 agentID（单卡 RP 的既有行为）。
func parseCastParam(raw, fallback string) []string {
	var out []string
	for _, id := range strings.Split(raw, ",") {
		if s := memorydir.SanitizeAgentID(id); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		if s := memorydir.SanitizeAgentID(fallback); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ── 角色扮演（酒馆模式）──────────────────────────────────────
//
// newRoleplayContextProvider 组装 RP 会话的上下文：导演契约 + 角色卡人设 +
// 世界书常驻条目。与助手链路的本质区别是**零工具**——模型不碰文件、命令、
// 网络，只负责演。技能库/工具索引/记忆索引一律不注入（省 token 且防模型
// 在戏里冒出一句「我来调个工具」）。
// 角色卡不存在或人设为空时，退化为通用 RP 导演（用户仍可直接跟「旁白」演）。

// rpDirectorContract 是 RP 模式的系统基底：怎么写戏、谁演谁、什么不能演。
// ⚠️ 注意：「严禁出戏 不提及工具」只针对演戏本身——剧情的编排动作（开战、
// 改状态）要走下面的「编排者工具」，那是你作为导演的职责，不是出戏。
const rpDirectorContract = `# 角色扮演模式
你正在和用户进行沉浸式角色扮演（TRPG/酒馆式）。你的职责是「演」，不是「帮忙」：
- 你扮演【角色卡】中的角色以及场景中出现的其他 NPC、旁白与环境；用户扮演主角（char），由用户决定主角说什么做什么。
- 永远不要替用户决定主角的言行、心理或选择；你的回复以角色的台词、动作描写（*斜体动作*）、环境叙述为主，停在把话头递给用户的节点。
- 保持人设一致：角色的自称、口癖、知识边界、性格、与主角的关系，全部以人设与世界书为准。人设没写的不要编成人设。
- 保持剧情连续性：已发生的对话、场景、时间线、角色状态（受伤/持有物品/好感度）都要延续，不要突然重置或出戏。
- 世界书条目是设定事实，与你的描述冲突时以世界书为准；触发到的设定要自然融入演出，不要复述条目原文。
- 描写要有画面感：五感、节奏、留白。对话短促真实，动作描写克制精准，不堆砌辞藻，不写「仿佛整个世界都…」这类空泛套话。
- 严禁出戏：不提及 AI、模型、提示词、规则、系统；不向用户解释你在做什么；即使用户问，也留在角色内回应。
- 回复长度以一场戏的呼吸为准：一般 2-6 段，不要灌水。

# 编排者工具（你是 Yosuri：既是演员也是导演）
你同时是这场戏的编排者。除了扮演角色，你手里有两只「笔」，**剧情推进到需要它们时直接调用，这不是出戏，这是导演的本职**：
- rp_battle_start：剧情出现遭遇战/伏击/决斗/Boss 战（用户说「开战」、「有敌人」、「打一架」，或你判断剧情到了战斗节点）时调用，把你扮演的所有角色和敌人拉进战场。敌人可选：slime 史莱姆 / wolf 灰狼 / goblin 哥布林 / skeleton 骷髅兵 / orc 兽人 / bandit 强盗头目 / dragon 幼龙。调用后战场弹出，由用户逐格操作回合（攻击/防御/技能），你只需在每回合结果出来后叙述战况。
- rp_set_stat：剧情里的数值变化要落到角色卡上——受伤（hp 减）、升级（level 加）、得到装备（equipment）、好感变化（affection）、印记（shield/marks）等。命令：set 设值 / add 增减 / custom 自定义参数 / world 补世界书设定。
用工具时继续用戏内的语言叙述（「史莱姆拦住了去路！」→ 调 rp_battle_start；「猫娘擦破了皮」→ 调 rp_set_stat add hp -5），不要把工具调用本身讲给用户听。`

func newRoleplayContextProvider(agentID string) *contextProvider {
	return newRoleplayContextProviderForCast([]string{agentID})
}

// newRoleplayContextProviderForCast 多角色同框：cast 里的每张卡都是模型要
// 扮演的角色（含旁白职责），用户在对面演主角。单卡时行为与旧版一致。
func newRoleplayContextProviderForCast(cast []string) *contextProvider {
	var cards []AgentCard
	for _, id := range cast {
		if c := GetAgentCard(id); c != nil {
			cards = append(cards, *c)
		}
	}
	var b strings.Builder
	if len(cards) == 0 {
		b.WriteString("# 角色卡\n（未指定角色卡：你扮演旁白与场景中出现的各类角色，由用户主导剧情走向。）\n\n")
	} else if len(cards) == 1 {
		b.WriteString("# 角色卡（你要扮演的角色）\n" + strings.TrimSpace(cards[0].Persona) + "\n\n")
	} else {
		b.WriteString("# 角色卡（本场戏你要同时扮演的角色）\n")
		b.WriteString("你扮演下列每一位角色，以及旁白与环境；用户扮演主角。台词前用「角色名：」标明说话人，各角色的自称、口癖、性格必须分明，不许混。\n\n")
		for _, c := range cards {
			b.WriteString("## " + c.Name + "\n" + strings.TrimSpace(c.Persona) + "\n\n")
		}
	}
	// 人物参数：有数值的角色把状态摆出来，模型演的时候才知道谁残血谁满状态。
	var statLines []string
	for _, c := range cards {
		if !c.Stats.HasStats() {
			continue
		}
		statLines = append(statLines, formatAgentStats(c.Name, c.Stats))
	}
	if len(statLines) > 0 {
		b.WriteString("# 角色当前状态（数值会随剧情变化，演出须与之一致）\n" + strings.Join(statLines, "\n") + "\n\n")
	}
	personaSection := b.String()
	worldSection := ""
	if lb := loadWorldBook(primaryCastID(cast)); lb != nil {
		if s := renderWorldBook(lb, ""); s != "" {
			worldSection = "\n\n# 世界书（设定事实）\n" + s
		}
	}
	return &contextProvider{
		rp:        true,
		activated: map[string]bool{},
		sections: []contextSection{
			{key: "system", content: rpDirectorContract + "\n\n" + personaSection + worldSection, stable: true},
		},
	}
}

// primaryCastID 取 cast 里第一个合法 id（世界书私有作用域按主角色卡走）。
func primaryCastID(cast []string) string {
	for _, id := range cast {
		if s := memorydir.SanitizeAgentID(id); s != "" {
			return s
		}
	}
	return ""
}

// formatAgentStats 把人物参数拼成一行模型可读的状态描述。
func formatAgentStats(name string, s AgentStats) string {
	var b strings.Builder
	b.WriteString("- " + name + "：")
	if s.Title != "" {
		b.WriteString("「" + s.Title + "」")
	}
	if s.Level > 0 {
		b.WriteString(fmt.Sprintf(" Lv.%d", s.Level))
	}
	if s.MaxHP > 0 {
		b.WriteString(fmt.Sprintf(" HP %d/%d", s.HP, s.MaxHP))
	}
	if s.Shield > 0 {
		b.WriteString(fmt.Sprintf(" 护盾%d", s.Shield))
	}
	if s.MaxMP > 0 {
		b.WriteString(fmt.Sprintf(" MP %d/%d", s.MP, s.MaxMP))
	}
	if s.Element != "" {
		b.WriteString(fmt.Sprintf(" 元素%s", s.Element))
	}
	for _, m := range s.Marks {
		if strings.TrimSpace(m.Name) == "" {
			continue
		}
		b.WriteString(" " + strings.TrimSpace(m.Name) + "印记" + strings.TrimSpace(m.Value))
	}
	if s.ATK > 0 || s.DEF > 0 {
		b.WriteString(fmt.Sprintf(" 攻%d 防%d", s.ATK, s.DEF))
	}
	if s.SPD > 0 {
		b.WriteString(fmt.Sprintf(" 速%d", s.SPD))
	}
	if s.MAG > 0 {
		b.WriteString(fmt.Sprintf(" 魔%d", s.MAG))
	}
	if s.Gold > 0 {
		b.WriteString(fmt.Sprintf(" 金币%d", s.Gold))
	}
	if s.Affection > 0 {
		b.WriteString(fmt.Sprintf(" 好感%d/100", s.Affection))
	}
	if len(s.Equipment) > 0 {
		b.WriteString(" 装备：" + strings.Join(s.Equipment, "、"))
	}
	for _, c := range s.Custom {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		b.WriteString(" " + strings.TrimSpace(c.Name) + "：" + strings.TrimSpace(c.Value))
	}
	return b.String()
}

// WithRPAtMention @呼人：把「用户点名谁」作为导演指令注到系统提示词最前。
// Yosuri 是编排者时提示它接管；普通角色时提示该角色优先回应、别人不抢戏。
func (p *contextProvider) WithRPAtMention(target string) *contextProvider {
	target = strings.TrimSpace(target)
	if target == "" {
		return p
	}
	mention := ""
	if strings.EqualFold(target, "yosuri") || strings.EqualFold(target, "Yosuri") {
		mention = "# 本回合用户点名：Yosuri（编排者）\n用户在叫你。这一回合请你以编排者身份响应：" +
			"可以调整演出节奏、修改人物参数、补充世界书设定，也可以让某个角色替你说戏。" +
			"如果用户只是叫你没别的话，就主动把戏接下去。\n\n"
	} else {
		mention = "# 本回合用户点名：" + target + "\n用户这句话是对「" + target +
			"」说的。请让 TA 优先回应；其余角色可以听到，但不要抢戏、不要替 TA 回话。\n\n"
	}
	p.sections = append([]contextSection{{key: "system", content: mention, stable: false}}, p.sections...)
	return p
}

// IsRoleplay 供调用层判断当前 provider 是否 RP 模式。
func (p *contextProvider) IsRoleplay() bool { return p.rp }


// OnInvoked 注册每轮收尾的落状态回调。
func (p *contextProvider) OnInvoked(fn func(round int, st roundState)) {
	p.onInvoked = fn
}

// memPart 一个待合并进记忆主体的分段，pri 越小优先级越高（越先保留）。
type memPart struct {
	pri     int
	content string
}

// combineMemoryWithBudget 按 token 预算把多个记忆分段拼成一个主体。
// 预算读用户档案 memory_token_budget（<=0 用默认 2200）；parts 构造时已按 pri 升序，
// 依次累加，超出预算的低优先段直接丢弃（宁缺毋滥，不硬塞）。
func combineMemoryWithBudget(parts []memPart) string {
	profile := loadUserProfile()
	budget := profile.MemoryTokenBudget
	if budget <= 0 {
		budget = 2200
	}
	var b strings.Builder
	used := 0
	for _, p := range parts {
		if strings.TrimSpace(p.content) == "" {
			continue
		}
		t := estimateTokenCount(p.content)
		if used > 0 && used+t > budget {
			break
		}
		b.WriteString("\n\n" + p.content)
		used += t
	}
	return strings.TrimSpace(b.String())
}

// SystemPrompt 按声明顺序拼出系统提示词（稳定段已在构造时排在前面）。
func (p *contextProvider) SystemPrompt() string {
	var b strings.Builder
	for _, s := range p.sections {
		b.WriteString(s.content)
	}
	return b.String()
}

// Breakdown 分类 token 占用，随 model_info 下发给前端 context 面板。
// 与 SystemPrompt 同源，不会再出现"加了一段忘了登记"的漂移。
func (p *contextProvider) Breakdown() map[string]int {
	out := make(map[string]int, len(p.sections)+1)
	for _, s := range p.sections {
		out[s.key] += estimateTokenCount(s.content)
	}
	// 常驻工具 schema 不在系统提示词里（走 tools 参数），但同样占 prompt_tokens，
	// 前端要看到它，否则分类之和永远对不上真实 prompt_tokens
	toolsJSON, _ := json.Marshal(p.Tools())
	out["tools"] += estimateTokenCount(string(toolsJSON))
	return out
}

// StaticSum 静态部分之和。下发 conversation_tokens 时要从真实 prompt_tokens 里
// 减掉它，否则前端把静态分类再加一遍就是双重计算。
func (p *contextProvider) StaticSum() int {
	sum := 0
	for _, v := range p.Breakdown() {
		sum += v
	}
	return sum
}

// Tools 本轮要发的 tools 数组：常驻工具 + 已激活的 Go 内置/MCP 工具。
// RP 模式只给 Yosuri 的笔（rp_set_stat 改人物参数/补世界书），其余工具一律
// 不暴露——模型只写戏，不碰文件/命令/网络，也就不会在角色里冒出一句
// 「我来读个文件」。
func (p *contextProvider) Tools() []map[string]any {
	if p.rp {
		defs := rpToolDefs()
		out := make([]map[string]any, 0, len(defs))
		for _, t := range defs {
			out = append(out, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Function.Name,
					"description": t.Function.Description,
					"parameters":  t.Function.Parameters,
				},
			})
		}
		return out
	}
	return buildCodeWorkflowTools(p.activated)
}

// Invoking 装配首轮消息：system + 会话历史 + 本次任务。
// 之后各轮由调用方在 msgs 上追加 assistant/tool 消息，provider 不再插手——
// 那些内容是循环产生的，硬要让 provider 拥有反而绕。
func (p *contextProvider) Invoking(history []DSMessage, task string) []map[string]any {
	built := buildChatMessages(p.SystemPrompt(), history, task)
	msgs := make([]map[string]any, len(built))
	for i, m := range built {
		msgs[i] = map[string]any{"role": m["role"], "content": m["content"]}
	}
	return msgs
}

// Invoked 一轮结束：把状态交给落状态回调（检查点）。
func (p *contextProvider) Invoked(round int, st roundState) {
	if p.onInvoked != nil {
		p.onInvoked(round, st)
	}
}

// ActivateTools 处理一次 load_tools 调用，返回给模型的结果文本和"是否有新工具被激活"。
// 激活后 Tools() 自动带上它们，调用方只需在 changed 时重新取一次。
func (p *contextProvider) ActivateTools(argsJSON string) (string, bool) {
	return handleLoadTools(argsJSON, p.activated)
}

// ActivatedTools 导出已激活集合，用于落检查点。
func (p *contextProvider) ActivatedTools() map[string]bool { return p.activated }

// RecordHotSkillUse 工作流成功收尾时调用：本轮热集技能被预加载且任务做完了，
// 补记一次使用。不补记的话模型因预加载而不再调 skill_view，分数自然衰减掉出
// 热集，形成"预加载杀死自己的入选理由"的死循环。
func (p *contextProvider) RecordHotSkillUse() {
	if p.hotSkillName != "" {
		recordSkillUse(p.hotSkillName)
	}
}

// IsActivated 判断某个工具是否已激活（用于动态按需加载：模型直接调用按需工具时，
// 主循环据此决定要不要自动激活并刷新 tools 数组）。
func (p *contextProvider) IsActivated(name string) bool { return p.activated[name] }

// RestoreActivatedTools 续跑时恢复中断前已加载的工具，
// 否则模型上一轮刚加载的工具突然消失，得再 load 一遍白费一轮。
func (p *contextProvider) RestoreActivatedTools(set map[string]bool) {
	if len(set) == 0 {
		return
	}
	p.activated = set
}

// projectWorkdirPrompt 把当前项目的 workdir.md（~/rescene_data/projects/<项目名>/workdir.md）
// 注入系统提示词，让 agent 每次会话一开始就了解「这个项目现在在做什么、关键上下文、
// 待办、约定」——跨对话的项目状态，避免失忆。与通用记忆（memorydir）互补：通用记忆是
// 用户/系统级常驻，workdir.md 是按项目隔离的。文件不存在时静默跳过（项目尚无笔记）。
// 路径隔离在 rescene_data 下，不污染 repo 本身。
func projectWorkdirPrompt() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	root := core.GetProjectRoot()
	if root == "" {
		return ""
	}
	proj := filepath.Base(root)
	path := filepath.Join(home, "rescene_data", "projects", proj, "workdir.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "" // 项目尚无 workdir.md，正常（agent 会在需要时写）
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	return "\n\n# 项目工作目录笔记（" + proj + "，跨对话保留）\n" + text
}
