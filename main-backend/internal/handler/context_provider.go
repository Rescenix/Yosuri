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
	// activated 已被 load_tools 激活的 Go 内置/MCP 工具，决定 Tools() 返回什么
	activated map[string]bool
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
	prefSection := ""
	if level >= 2 {
		if pref := memorydir.ReadRaw("preferences"); pref != "" {
			prefSection = "\n\n# 用户偏好（亲密等级解锁自动回填）\n" + pref
		}
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

	return &contextProvider{
		activated: map[string]bool{},
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

			// —— 易变段：一变就让它后面的缓存作废，所以一律排在最后 ——
			{key: "skill", content: skillLibraryPrompt() + autoLoadedSkillsPrompt(task)}, // 索引 + 当前任务确定性预加载
			{key: "memory", content: memorySection},                                      // 每写一次记忆就变
			{key: "memory", content: pinnedSection},                                      // 常驻记忆：memory_pin 写入，跨对话常驻
			{key: "memory", content: handoffSection},                                     // 会话交接工作态：memory_handoff 写入
			{key: "memory", content: intimacySection},                                    // 亲密等级：关系等级注入，驱动语气与记忆行为
			{key: "memory", content: prefSection},                                        // 偏好自动回填：亲密度 ≥100 无条件注入
			{key: "memory", content: workdirSection},                                     // 项目级 workdir.md，会话开始即注入，跨对话不失业
			{key: "memory", content: taskMemory},                                         // 反向链接联想召回：命中 + 1跳展开
			{key: "memory", content: knowledgeSection},                                   // 外挂知识库 RAG：检索召回相关片段
			// 自定义指令归到 system 桶（同属"给模型的指令"，且只有十几 tok，
			// 单开一个桶不值得改前端契约）。原来它根本没进 breakdown，是个漏登记。
			{key: "system", content: userInstructionsPrompt()},
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

// OnInvoked 注册每轮收尾的落状态回调。
func (p *contextProvider) OnInvoked(fn func(round int, st roundState)) {
	p.onInvoked = fn
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
func (p *contextProvider) Tools() []map[string]any {
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
// 待办、约定」——跨对话的项目状态，避免失忆。与全局 MEMORY.md 互补：MEMORY.md 是
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
