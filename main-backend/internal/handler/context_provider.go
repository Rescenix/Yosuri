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
	"strings"

	"backend/internal/agent"
	"backend/internal/swiftnet"
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
	// activated 已被 load_tools 激活的 MCP 工具，决定 Tools() 返回什么
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
func newWorkflowContextProvider() *contextProvider {
	memoryInject := swiftnet.Default().UnconditionalInject()
	memorySection := ""
	if memoryInject != "" {
		memorySection = "\n\n# 长期记忆（无条件注入，身份/工作态/收件箱）\n" + memoryInject
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
			// 工具索引只在 MCP server 增删时变（很少），算稳定段；
			// 完整 schema 靠 load_tools 按需取，见 tool_ondemand.go。
			// key 用 "tools" 是有意的：前端 contextBreakdown.js 只认
			// system/subagent/skill/memory/tools 五个桶，索引归到工具桶里，
			// 免得凭空多一个前端会丢掉的 key，害「分类之和 ≈ prompt_tokens」对不上。
			{key: "tools", content: mcpToolIndexPrompt(), stable: true},

			// —— 易变段：一变就让它后面的缓存作废，所以一律排在最后 ——
			{key: "skill", content: skillLibraryPrompt()}, // 每次任务成功后可能新增技能
			{key: "memory", content: memorySection},       // 每写一次记忆就变
			// 自定义指令归到 system 桶（同属"给模型的指令"，且只有十几 tok，
			// 单开一个桶不值得改前端契约）。原来它根本没进 breakdown，是个漏登记。
			{key: "system", content: userInstructionsPrompt()},
		},
	}
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

// Tools 本轮要发的 tools 数组：常驻工具 + 已激活的 MCP 工具。
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

// RestoreActivatedTools 续跑时恢复中断前已加载的工具，
// 否则模型上一轮刚加载的工具突然消失，得再 load 一遍白费一轮。
func (p *contextProvider) RestoreActivatedTools(set map[string]bool) {
	if len(set) == 0 {
		return
	}
	p.activated = set
}
