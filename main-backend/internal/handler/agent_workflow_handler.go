package handler

// 四态机 Code 工作流 —— GET /api/code/workflow?task=...&session_id=...
//
// 对标 OpenCode 的"思考→意图→操作→结果"极简交互流，通过 SSE 推送：
//
//   workflow_start  {workflow_id, task}
//   model_info      {name, vision, context_window, reasoning}  // 本轮实际承接的 backend 能力元数据，每个工作流只发一次
//   thinking        {content}   // 模型 reasoning_content 增量（模型支持时才有）
//   intent          {content}   // 叙述文本增量（工具调用前的意图说明 / 最终回答）
//   action          {id, name, args}         // args 是真实 JSON 字符串
//   result          {id, name, ok, output}   // 工具执行结果
//   workflow_done   {status, final_output, input_tokens, output_tokens}
//   flow_error      {message}   // 命名避开 EventSource 原生 error 事件
//
// 与 /api/workflow/run 的旧契约完全独立：这里字段用真实 JSON 类型（bool/number），
// args 用真实 JSON，前端由 AgentWorkflowPanel.vue + useAgentWorkflow.js 消费。

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"backend/internal/agent"
	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

const (
	codeWorkflowMaxRounds = 20
	codeResultMaxChars    = 10000
	// codeRepeatCallLimit 同一 工具名+参数 在一个工作流里最多真实执行几次；
	// 超出即熔断（回提示不再真跑），防止模型原地打转烧满轮次。
	codeRepeatCallLimit = 2
)

// historyLimitFor 按模型上下文能力自适应历史窗口。
// 原来所有链路共用 maxHistoryMessages=10——那是给 8K 小模型的保守值，
// 对 100 万上下文的 Gemini 等于白白把对话记忆砍掉；前端"上下文占用不随对话增长"
// 的观感也源于此（聊到第 10 条 prompt 就到顶了）。
func historyLimitFor(contextWindow int) int {
	switch {
	case contextWindow >= 200000:
		return 60
	case contextWindow >= 100000:
		return 40
	case contextWindow >= 32000:
		return 24
	case contextWindow > 0:
		return 12
	default:
		return maxHistoryMessages
	}
}

// conversationTokens 把真实 prompt_tokens 里属于"对话"的部分摘出来。
// input_tokens 是上游返回的 usage.prompt_tokens——它已经包含系统提示词/工具定义/
// 记忆/技能/子代理定义。前端面板若再把这些静态分类加一遍就是双重计算，
// 所以这里减掉静态部分后再下发，保证 分类之和 ≈ 真实 prompt_tokens。
func conversationTokens(inputTokens, staticSum int) int {
	if n := inputTokens - staticSum; n > 0 {
		return n
	}
	return 0
}

// codeSSEMu 串行化 SSE 写入：子代理 goroutine 会并发发出 subagent_* 事件，
// gin 的 ResponseWriter 不是并发安全的。全局锁跨请求也会串行，但每次写都是
// 微秒级 buffer 操作，不构成瓶颈——比 per-request 锁结构简单得多。
var codeSSEMu sync.Mutex

func writeCodeSSE(c *gin.Context, event string, data map[string]any) {
	codeSSEMu.Lock()
	defer codeSSEMu.Unlock()
	data["type"] = event
	b, _ := json.Marshal(data)
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, b)
	c.Writer.Flush()
}

func truncateChars(s string, max int) string {
	if len(s) <= max {
		return s
	}
	// 按 rune 边界截断，避免切碎多字节 UTF-8
	runes := []rune(s)
	total := 0
	for i, r := range runes {
		total += len(string(r))
		if total > max {
			return string(runes[:i]) + "\n...[已截断]"
		}
	}
	return s
}

// HandleCodeWorkflow GET /api/code/workflow — 四态机 SSE 工作流
func (r *WorkflowRunner) HandleCodeWorkflow(c *gin.Context) {
	// resume=<workflow_id>：从上次落盘的检查点接着跑（后端重启/SSE 断线后的续跑入口）。
	// 检查点里存了 task/mode/model 等全部启动参数，所以续跑时这些 query 参数可以不带。
	resumeID := strings.TrimSpace(c.Query("resume"))
	var resumed *workflowCheckpoint
	if resumeID != "" {
		resumed = loadWorkflowCheckpoint(resumeID)
		if resumed == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "检查点不存在或已过期: " + resumeID})
			return
		}
	}

	task := strings.TrimSpace(c.Query("task"))
	if resumed != nil && task == "" {
		task = resumed.Task
	}
	if task == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task 参数必填"})
		return
	}
	sessionID := c.Query("session_id")
	if resumed != nil && sessionID == "" {
		sessionID = resumed.SessionID
	}
	// mode: yolo(全自动,默认) / ask(危险工具每步问)。不传或非法值(含旧 plan)按 yolo 处理。
	mode := strings.ToLower(c.Query("mode"))
	if mode == "" && resumed != nil {
		mode = resumed.Mode
	}
	if mode != "ask" {
		mode = "yolo"
	}

	// 模型路由链：前端选了具体模型就精确路由到那一个；否则走用户配置>env DeepSeek>
	// 免费池的全链。注意本地兜底已移除（8186699e），一个 Key 都没配时链会是空的，
	// 由 streamRouterRound 给出"去配 Key"的明确报错。
	openID, model := c.Query("openid"), c.Query("model")
	effort := c.Query("effort") // "low"/"medium"/"high"，只有 backend.Reasoning=true 时才真的生效
	if resumed != nil {
		// 续跑沿用原来的模型：中途换模型会让已有的 tool_calls 历史落到另一套
		// 工具调用格式上，不如从头跑一遍干净。
		openID, model, effort = resumed.OpenID, resumed.Model, resumed.Effort
	}
	backends := resolveBackends(openID, model)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 续跑复用原 workflow_id：检查点文件跟着它走，反复中断也只有一份。
	workflowID := agent.NewWorkflowID()
	if resumed != nil {
		workflowID = resumed.WorkflowID
	}
	writeCodeSSE(c, "workflow_start", map[string]any{
		"workflow_id": workflowID, "task": task, "mode": mode,
		"resumed": resumed != nil, "resumed_round": func() int {
			if resumed != nil {
				return resumed.Round
			}
			return 0
		}(),
	})

	// 审批等待器：整个工作流生命周期共用一个（channel 按 approval id 区分，不会跨轮串）。
	// 注册进全局 registry，供独立的 POST /api/code/workflow/approve 跨请求唤醒。
	waiter := newApprovalWaiter()
	registerApprovalWaiter(workflowID, waiter)
	defer unregisterApprovalWaiter(workflowID)

	// 上下文装配全部交给 ContextProvider（见 context_provider.go）：
	// 系统提示词分段声明、稳定段排前面（前缀缓存友好）、分类占用与提示词同源、
	// 按需加载的工具激活集也归它管。SwiftNet 的无条件记忆注入是其中一段。
	provider := newWorkflowContextProvider()
	contextBreakdown := provider.Breakdown()
	staticSum := provider.StaticSum()
	tools := provider.Tools()

	// 之前这里每次都是只有 system+当前 task 的白板，session_id 传了但从没读过——
	// LLM 完全不知道上一条消息说了什么。跟 chat_stream 那条老路径一样，从
	// sessionStore 捞这个会话的历史拼进去（窗口按模型上下文能力自适应，
	// 见 historyLimitFor——固定 10 条会让大上下文模型的对话记忆被白白砍掉），
	// 工作流结束后再把这一轮的 user/assistant 写回去，下一条消息才能接上下文。
	histLimit := maxHistoryMessages
	if len(backends) > 0 {
		histLimit = historyLimitFor(backends[0].ContextWindow)
	}
	history := truncateHistory(r.chatHandler.sessionStore.Get(sessionID), histLimit)
	msgs := provider.Invoking(history, task)
	historyChars := 0
	for _, m := range msgs {
		if s, ok := m["content"].(string); ok {
			historyChars += len(s)
		}
	}

	var transcript []string // 动作摘要，供技能生成
	// flowBlocks 是这次工作流的可视化轨迹，与推给前端的 intent/action/result 事件同源。
	// 收尾时挂到 assistant 消息上落盘，刷新页面后聊天记录里工具行和展开详情还在
	// （之前只存最终那段文本，历史里的工具调用一刷新就蒸发）。
	var flowBlocks []FlowBlock
	// 循环熔断：同一 工具名+参数 的调用次数。与具体模型无关的护栏——
	// 模型抽风（看不见工具结果、或单纯钻牛角尖）时不该白烧满 codeWorkflowMaxRounds 轮。
	callSignatureCount := map[string]int{}
	inputTokens := historyChars / 4
	outputTokens := 0
	callSeq := 0
	startRound := 0
	modelInfoSent := false

	// 续跑：整体接管上面刚拼好的白板状态。msgs 用检查点里的完整对话
	// （含中断前所有 tool_calls 和工具结果），模型醒来就知道自己干到哪了。
	if resumed != nil {
		msgs = resumed.Msgs
		transcript = resumed.Transcript
		if resumed.CallSigCount != nil {
			callSignatureCount = resumed.CallSigCount
		}
		callSeq = resumed.CallSeq
		provider.RestoreActivatedTools(resumed.ActivatedTools)
		tools = provider.Tools() // 带回中断前已加载的工具，免得再 load 一遍白费一轮
		inputTokens = resumed.InputTokens
		outputTokens = resumed.OutputTokens
		startRound = resumed.Round
		log.Printf("🔁 [续跑] workflow=%s 从第 %d 轮恢复，历史 %d 条消息", workflowID, startRound, len(msgs))
	}

	// Invoked 钩子：每轮收尾把状态落成检查点。启动参数（task/mode/model…）由这里的
	// 闭包捕获，轮次内变化的 msgs/transcript/token 由 roundState 传入——
	// provider 不假装拥有循环的状态，只提供"一轮结束了"这个落点。
	provider.OnInvoked(func(round int, st roundState) {
		saveWorkflowCheckpoint(&workflowCheckpoint{
			WorkflowID: workflowID, SessionID: sessionID, OpenID: openID,
			Task: task, Mode: mode, Model: model, Effort: effort,
			Round: round, Msgs: st.msgs, Transcript: st.transcript,
			CallSigCount: st.callSigCount, CallSeq: st.callSeq,
			ActivatedTools: provider.ActivatedTools(),
			InputTokens:    st.inputTokens, OutputTokens: st.outputTokens,
		})
	})
	// checkpoint 收拢 roundState 的组装，免得两个调用点各写一遍。
	checkpoint := func(round int) {
		provider.Invoked(round, roundState{
			msgs: msgs, transcript: transcript, callSigCount: callSignatureCount,
			callSeq: callSeq, inputTokens: inputTokens, outputTokens: outputTokens,
		})
	}

	// 触发压缩用的上下文窗口：优先取模型实报的，取不到用兜底常量
	ctxWindow := estimatedContextWindow
	if len(backends) > 0 && backends[0].ContextWindow > 0 {
		ctxWindow = backends[0].ContextWindow
	}

	for round := startRound; round < codeWorkflowMaxRounds; round++ {
		if c.Request.Context().Err() != nil {
			return // 客户端断开
		}

		// 上下文感知压缩：真实 prompt_tokens 超窗口 80% 时，把早期轮次折叠成任务相关
		// 摘要，腾出预算继续跑（见 context_compress.go）。inputTokens 是上一轮上游返回的
		// 真实 prompt_tokens，比字符估算准。压缩失败会原样返回，不影响主流程。
		if newMsgs, cr := r.compressContextIfNeeded(c.Request.Context(), backends, msgs, task, inputTokens, ctxWindow); cr.Compressed {
			msgs = newMsgs
			writeCodeSSE(c, "context_compressed", map[string]any{
				"folded_messages": cr.FoldedMsgs,
				"before_chars":    cr.BeforeChars,
				"after_chars":     cr.AfterChars,
			})
			log.Printf("🗜️ [压缩] workflow=%s 第 %d 轮：折叠 %d 条消息 %d→%d 字符",
				workflowID, round, cr.FoldedMsgs, cr.BeforeChars, cr.AfterChars)
		}

		content, calls, inTok, outTok, usedBackend, err := r.streamRouterRound(c, backends, msgs, tools, effort)
		// inTok 优先用上游真实 prompt_tokens；为 0 时退化为历史字符/4 估算（与四态机口径一致）
		if inTok > 0 {
			inputTokens = inTok
		}
		outputTokens += outTok
		// 只在第一轮实际承接请求后发一次——同一个工作流后续轮次不会换 backend，
		// 前端只需要知道"这次对话用的是哪个模型、它能不能识图/支持多大上下文"一次就够
		if usedBackend != nil && !modelInfoSent {
			modelInfoSent = true
			writeCodeSSE(c, "model_info", map[string]any{
				"name": usedBackend.Name, "vision": usedBackend.Vision,
				"context_window": usedBackend.ContextWindow, "reasoning": usedBackend.Reasoning,
				"context_breakdown": contextBreakdown,
			})
		}
		if err != nil {
			// 上游挂了属于可恢复失败——保留检查点，前端可以带 resume=<id> 原地重试，
			// 不必把已经跑完的十几轮工具再跑一遍。
			checkpoint(round)
			writeCodeSSE(c, "flow_error", map[string]any{"message": err.Error()})
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "failed", "final_output": "任务执行失败: " + err.Error(),
				"input_tokens": inputTokens, "output_tokens": outputTokens,
				"conversation_tokens": conversationTokens(inputTokens, staticSum),
				"resumable":           true, "workflow_id": workflowID,
			})
			return
		}

		// 没有工具调用 → 最终回答，收尾
		if len(calls) == 0 {
			deleteWorkflowCheckpoint(workflowID)
			cleanupToolOutputSpills(workflowID)
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "completed", "final_output": content,
				"input_tokens": inputTokens, "output_tokens": outputTokens,
				"conversation_tokens": conversationTokens(inputTokens, staticSum),
			})
			if sessionID != "" {
				if content != "" {
					flowBlocks = append(flowBlocks, FlowBlock{Type: "intent", Text: content})
				}
				r.chatHandler.sessionStore.Append(sessionID, DSMessage{Role: "user", Content: task})
				r.chatHandler.sessionStore.Append(sessionID, DSMessage{
					Role: "assistant", Content: content, Blocks: flowBlocks,
				})
			}
			go generateSkillAsync(task, transcript)
			return
		}

		// 这一轮模型在调工具之前说的话，也是轨迹的一部分（"我先看看这个文件"）
		if content != "" {
			flowBlocks = append(flowBlocks, FlowBlock{Type: "intent", Text: content})
		}

		// action 事件（args 为真实 JSON）
		for i := range calls {
			callSeq++
			if calls[i].ID == "" {
				calls[i].ID = fmt.Sprintf("call_%d", callSeq)
			}
			writeCodeSSE(c, "action", map[string]any{
				"id": calls[i].ID, "name": calls[i].Function.Name, "args": calls[i].Function.Arguments,
			})
		}

		// 执行：dispatch_agent 并行（雨燕群），其余顺序。
		// emit 回调把子代理生命周期事件实时写进 SSE 流（写入端有锁，跨 goroutine 安全）
		emit := func(event string, data map[string]any) {
			if c.Request.Context().Err() == nil {
				writeCodeSSE(c, event, data)
			}
		}
		// 熔断判定：同一签名（工具名+参数）第 3 次及以后不再真跑，直接回一条提示当结果。
		// 结果必然与前两次相同，重跑纯属浪费；把"别再调了"写进历史，给模型一个转向的机会。
		blocked := make([]bool, len(calls))
		handled := make([]string, len(calls)) // 非空 = 已在本层处理完，不进 executeCodeCalls
		toRun := make([]core.ToolCall, 0, len(calls))
		runIdx := make([]int, 0, len(calls))
		allBlocked := true
		for i, tc := range calls {
			if shouldBlockRepeat(callSignatureCount, tc.Function.Name, tc.Function.Arguments, codeRepeatCallLimit) {
				blocked[i] = true
				continue
			}
			allBlocked = false
			// load_tools 是纯上下文操作（取 schema + 激活），没有副作用也不需要审批，
			// 在这层直接办掉，不进工具执行链。
			if tc.Function.Name == loadToolsToolName {
				out, changed := provider.ActivateTools(tc.Function.Arguments)
				handled[i] = out
				if changed {
					// 下一轮的 tools 数组带上刚激活的工具，模型才能真正调它
					tools = provider.Tools()
				}
				continue
			}
			toRun = append(toRun, calls[i])
			runIdx = append(runIdx, i)
		}

		// 审批等待器：仅 ask 模式需要；yolo 模式下 executeCodeCalls 内部会直接跳过拦截。
		// 使用整个 workflow 共用的 waiter（按 approval id 区分），不每轮新建。
		results := make([]codeExecResult, len(calls))
		for i := range results {
			switch {
			case blocked[i]:
				results[i] = codeExecResult{
					failed: true,
					output: fmt.Sprintf("已阻止：%s 用完全相同的参数连续调用了 %d 次，结果不会变化。请勿重复调用，改用已有结果作答或换一个思路。",
						calls[i].Function.Name, codeRepeatCallLimit),
				}
			case handled[i] != "":
				results[i] = codeExecResult{output: handled[i]}
			}
		}
		if len(toRun) > 0 {
			ran := r.executeCodeCalls(c, backends, toRun, emit, mode, waiter, sessionID, workflowID)
			for k, idx := range runIdx {
				results[idx] = ran[k]
			}
		}

		// 对话历史追加 assistant(tool_calls)
		var dsCalls []map[string]any
		for _, tc := range calls {
			dsCalls = append(dsCalls, map[string]any{
				"id": tc.ID, "type": "function",
				"function": map[string]any{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
			})
		}
		msgs = append(msgs, map[string]any{"role": "assistant", "content": content, "tool_calls": dsCalls})

		// result 事件 + tool 消息
		for i, tc := range calls {
			// 进上下文的是压缩版（首尾保留 + 全文落盘）；前端 result 事件给完整输出，
			// 用户看工具卡片时不该被模型的 token 预算限制视野。
			full := truncateChars(results[i].output, codeResultMaxChars)
			out := compactToolOutput(workflowID, tc.ID, results[i].output)
			writeCodeSSE(c, "result", map[string]any{
				"id": tc.ID, "name": tc.Function.Name, "ok": !results[i].failed, "output": full,
			})
			status := "ok"
			if results[i].failed {
				status = "error"
			}
			flowBlocks = append(flowBlocks, FlowBlock{
				Type: "tool", Name: tc.Function.Name, Args: tc.Function.Arguments,
				Output: full, Status: status,
			})
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": tc.ID, "content": out})
			inputTokens += len(out) / 4
			transcript = append(transcript, fmt.Sprintf("%s(%s) => %s",
				tc.Function.Name, truncateChars(tc.Function.Arguments, 300), truncateChars(results[i].output, 200)))
		}
		inputTokens += len(content) / 4

		// 落盘断点：此刻这一轮的工具全部执行完、结果已进 msgs，没有半途状态，
		// 是唯一安全的恢复边界。后端从这里被杀掉，续跑就从下一轮问模型开始。
		checkpoint(round + 1)

		// 整轮调用全被熔断 → 模型已经在原地打转，提示也没拉回来，直接收尾，
		// 而不是陪它空转到 codeWorkflowMaxRounds。
		if allBlocked {
			deleteWorkflowCheckpoint(workflowID)
			cleanupToolOutputSpills(workflowID)
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "failed", "final_output": "检测到模型重复调用同一工具且无进展，已中止任务。",
				"input_tokens": inputTokens, "output_tokens": outputTokens,
				"conversation_tokens": conversationTokens(inputTokens, staticSum),
			})
			return
		}
	}

	// 轮次耗尽属于终态失败（续跑也只会立刻再撞上限），检查点没有保留价值。
	deleteWorkflowCheckpoint(workflowID)
	cleanupToolOutputSpills(workflowID)
	writeCodeSSE(c, "workflow_done", map[string]any{
		"status": "failed", "final_output": fmt.Sprintf("超过最大迭代轮数(%d)，任务中止", codeWorkflowMaxRounds),
		"input_tokens": inputTokens, "output_tokens": outputTokens,
		"conversation_tokens": conversationTokens(inputTokens, staticSum),
	})
}

// HandleCodeWorkflowCheckpoints GET /api/code/workflow/checkpoints?session_id=
// 列出可续跑的中断工作流；前端据此显示「上次有个任务没跑完」并带 resume=<id> 重连。
func (r *WorkflowRunner) HandleCodeWorkflowCheckpoints(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"checkpoints": listWorkflowCheckpoints(c.Query("session_id"))})
}

// HandleCodeWorkflowCheckpointDelete DELETE /api/code/workflow/checkpoints/:id
// 用户明确放弃某个中断任务时清掉它（不删也会被 24h TTL 收走）。
func (r *WorkflowRunner) HandleCodeWorkflowCheckpointDelete(c *gin.Context) {
	deleteWorkflowCheckpoint(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type codeExecResult struct {
	output string
	failed bool
}

// shouldBlockRepeat 熔断判定：同一签名（工具名+参数）真实执行次数达到 limit 后，
// 第 limit+1 次及以后返回 true（本层拦截，不再真跑）。会自增计数。
// 抽成纯函数是为了能脱离整个 SSE handler 单测——熔断是"模型抽风时的护栏"，
// 健康模型看得见工具结果就不会重复调，实况里几乎不触发，只能靠单测覆盖。
func shouldBlockRepeat(counts map[string]int, name, args string, limit int) bool {
	sig := name + "|" + args
	counts[sig]++
	return counts[sig] > limit
}

// executeCodeCalls 执行一轮里的所有工具调用。
// dispatch_agent（雨燕子代理）用 goroutine 并行跑，其余工具在当前 goroutine 顺序执行，
// 全部完成后按原始顺序返回，保证 result 事件和 tool 消息的顺序稳定。
//
// 审批：mode=ask 且工具属于危险类（写盘/执行命令/MCP 文件写删）时，执行前通过 SSE 推
// approval_request 并阻塞等批准；yolo 模式或工具不危险则直接执行。会话已设 don't-ask-again
// 的同款工具也直接执行。
func (r *WorkflowRunner) executeCodeCalls(c *gin.Context, backends []RouterBackend, calls []core.ToolCall, emit func(string, map[string]any), mode string, waiter *approvalWaiter, sessionID string, workflowID string) []codeExecResult {
	results := make([]codeExecResult, len(calls))
	var wg sync.WaitGroup

	// maybeRequestApproval 在 ask 模式下对危险工具发起审批拦截；返回是否允许执行。
	// 非危险工具 / yolo 模式 / 已设 don't-ask-again → 直接返回 true（放行）。
	maybeRequestApproval := func(tc core.ToolCall) bool {
		if mode == "yolo" || !isDangerousTool(tc.Function.Name) {
			return true
		}
		if r.shouldAutoApprove(sessionID, tc.Function.Name) {
			return true
		}
		// 登记 + 推 SSE 事件 + 阻塞等批准。approval id 编码 workflowID::callID，
		// 让独立的 approve 端点能反解出 waiter。
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("approval_%d", time.Now().UnixNano())
		}
		approvalID := workflowID + "::" + id
		waiter.expect(approvalID)
		writeCodeSSE(c, "approval_request", map[string]any{
			"id":   approvalID,
			"tool": tc.Function.Name,
			"args": tc.Function.Arguments,
			"mode": mode,
		})
		// 客户端断开则中止执行
		allowed := waiter.wait(approvalID, c.Request.Context().Done())
		return allowed
	}

	for i, tc := range calls {
		if tc.Function.Name == "dispatch_agent" {
			wg.Add(1)
			go func(i int, tc core.ToolCall) {
				defer wg.Done()
				out, err := runSubAgent(c.Request.Context(), backends, tc.ID, tc.Function.Arguments, emit)
				if err != nil {
					results[i] = codeExecResult{output: "子代理执行失败: " + err.Error(), failed: true}
					return
				}
				results[i] = codeExecResult{output: out}
			}(i, tc)
		}
	}

	// 主循环：顺序执行非 dispatch_agent 的工具调用
	for i, tc := range calls {
		name := tc.Function.Name
		if name == "dispatch_agent" {
			continue
		}
		if !maybeRequestApproval(tc) {
			results[i] = codeExecResult{output: "用户未批准执行 " + name + "，已跳过", failed: true}
			continue
		}
		if strings.HasPrefix(name, "mcp__") {
			// MCP edit_file：执行前读文件记下行号，执行后补到结果里
			var preEditLine int
			if name == "mcp__fs__edit_file" {
				preEditLine = r.calcEditStartLine(tc.Function.Arguments)
			}
			out, err := callMCPTool(name, tc.Function.Arguments)
			if err != nil {
				results[i] = codeExecResult{output: "MCP 工具失败: " + err.Error(), failed: true}
			} else {
				if name == "mcp__fs__edit_file" && preEditLine > 0 && !strings.Contains(out, "第") {
					out = fmt.Sprintf("%s（第 %d 行）", out, preEditLine)
				}
				results[i] = codeExecResult{output: out}
			}
			continue
		}
		if name == "search_memory" {
			out, failed := r.searchMemory(tc.Function.Arguments)
			results[i] = codeExecResult{output: out, failed: failed}
			continue
		}
		res, err := core.ExecuteToolCall(tc)
		if err != nil {
			results[i] = codeExecResult{output: err.Error(), failed: true}
			continue
		}
		results[i] = codeExecResult{output: res.Content, failed: res.Failed}
	}

	wg.Wait()
	return results
}

// searchMemory 实现 search_memory 工具：在 workflow 链里优先在全部历史会话里做
// 正则检索（scope=sessions，默认），找不到或显式 scope=memory 时退回 SwiftNet 记忆卡片。
// 只拦截 workflow 这条链；chat 链仍走 core.ExecuteToolCall 里的原 memory 逻辑。
func (r *WorkflowRunner) searchMemory(argsJSON string) (string, bool) {
	var args struct {
		Query     string `json:"query"`
		Scope     string `json:"scope"`
		Mode      string `json:"mode"`
		ID        string `json:"id"`
		SessionID string `json:"session_id"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "search_memory 参数解析失败: " + err.Error(), true
	}
	if args.Scope == "" {
		args.Scope = "sessions"
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}

	if args.Scope == "memory" {
		// 退回 SwiftNet 长期记忆卡片
		res, err := core.ExecuteToolCall(core.ToolCall{
			Function: core.ToolCallFunc{Name: "search_memory", Arguments: argsJSON},
		})
		if err != nil {
			return err.Error(), true
		}
		return res.Content, res.Failed
	}

	// scope == "sessions"：全量检索历史会话消息
	if args.Query == "" {
		return "sessions 检索需要提供 query（正则表达式）", true
	}
	// 编译正则；失败则退化为大小写不敏感子串匹配
	re, err := regexp.Compile(args.Query)
	if err != nil {
		re = regexp.MustCompile(regexp.QuoteMeta(args.Query))
	}

	store := r.chatHandler.sessionStore
	all := store.AllSessions()
	var hits []string
	seen := 0
	for sid, msgs := range all {
		if args.SessionID != "" && sid != args.SessionID {
			continue
		}
		for _, m := range msgs {
			text := m.Content
			if text == "" {
				text = m.ReasoningContent
			}
			if text == "" {
				continue
			}
			idx := re.FindStringIndex(text)
			if idx == nil {
				continue
			}
			// 命中片段：前后各 60 rune 上下文
			r0, r1 := idx[0], idx[1]
			lo, hi := r0-60, r1+60
			runes := []rune(text)
			if lo < 0 {
				lo = 0
			}
			if hi > len(runes) {
				hi = len(runes)
			}
			snippet := string(runes[lo:hi])
			ts := m.Timestamp.Format("2006-01-02 15:04")
			hits = append(hits, fmt.Sprintf("• [%s] %s/%s: …%s…", sid, m.Role, ts, snippet))
			seen++
			if seen >= args.Limit {
				break
			}
		}
		if seen >= args.Limit {
			break
		}
	}

	if len(hits) == 0 {
		return fmt.Sprintf("未在历史会话中找到匹配 %q 的内容", args.Query), false
	}
	return fmt.Sprintf("在全部会话中正则匹配 %q，命中 %d 处（session_id 可传给 scope=sessions 的 session_id 或 memory 检索）：\n%s",
		args.Query, len(hits), strings.Join(hits, "\n")), false
}

// hardcodeFileTools 是已被 MCP filesystem 取代、从工作流 agent 工具集移除的
// 内置文件操作工具。agent 改用 mcp__fs__*，避免两套实现并存。
var hardcodeFileTools = map[string]bool{
	"read_file":       true,
	"write_file":      true,
	"edit_file":       true,
	"list_dir":        true,
	"execute_command": true,
}

// buildCodeWorkflowTools 已挪到 tool_ondemand.go：MCP 工具改为按需加载，
// 不再无条件全量塞进每一轮请求（实测省 6000+ tok/轮）。

// calcEditStartLine 在 MCP edit_file 执行前读文件，计算 oldText 的起始行号。
func (r *WorkflowRunner) calcEditStartLine(argsJSON string) int {
	var args struct {
		Path  string `json:"path"`
		Edits []struct {
			OldText string `json:"oldText"`
		} `json:"edits"`
		OldText  string `json:"oldText"`
		OldStr   string `json:"old_string"` // 模型偶尔把这个工具当内置 edit_file 的扁平 schema 调，见 normalizeMCPEditArgs
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return 0
	}
	// 取第一个 edit 的 oldText
	oldStr := args.OldText
	if oldStr == "" {
		oldStr = args.OldStr
	}
	if len(args.Edits) > 0 && args.Edits[0].OldText != "" {
		oldStr = args.Edits[0].OldText
	}
	if oldStr == "" || args.Path == "" {
		return 0
	}
	// 读文件找行号
	fullPath := args.Path
	if !strings.HasPrefix(fullPath, "/") && !strings.Contains(fullPath, ":") {
		fullPath = core.GetProjectRoot() + "/" + fullPath
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return 0
	}
	content := string(data)
	idx := strings.Index(content, oldStr)
	if idx < 0 {
		return 0
	}
	return strings.Count(content[:idx], "\n") + 1
}
