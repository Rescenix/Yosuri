package handler

// 四态机 Code 工作流 —— GET /api/code/workflow?task=...&session_id=...
//
// 对标 OpenCode 的"思考→意图→操作→结果"极简交互流，通过 SSE 推送：
//
//   workflow_start  {workflow_id, task}
//   model_info      {name, vision, context_window, reasoning}  // 本轮实际承接的 backend 能力元数据，每个工作流只发一次
//   thinking        {content}   // 模型 reasoning_content 增量（模型支持时才有）
//   intent          {content}   // 叙述文本增量（工具调用前的意图说明 / 最终回答）
//   action_delta    {id, name, args_delta} // 工具参数生成中的增量；只用于前端预览，不执行
//   action          {id, name, args}         // args 是真实 JSON 字符串
//   result          {id, name, ok, output}   // 工具执行结果
//   workflow_done   {status, final_output, input_tokens, output_tokens}
//   flow_error      {message}   // 命名避开 EventSource 原生 error 事件
//   steering_injected {message}   // POST /api/code/workflow/steer 投进来的中途插话已生效
//   preview_open    {url}       // 检测到前端文件改动，前端据此自动弹出预览面板（每个工作流一次）
//
// 与 /api/workflow/run 的旧契约完全独立：这里字段用真实 JSON 类型（bool/number），
// args 用真实 JSON，前端由 AgentWorkflowPanel.vue + useAgentWorkflow.js 消费。

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"backend/internal/agent"
	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

const (
	// codeWorkflowMaxRounds 现在只是兜底上限——真正约束成本的是下面的 token 预算
	// （见 codeWorkflowExhausted）。复杂任务经常真实需要几十轮，20 轮硬顶会把没
	// 跑完的正常任务错杀成"失败"；把它调宽松，让 token 预算做主要限制更贴合实际成本。
	codeWorkflowMaxRounds = 60
	// codeWorkflowMaxTokensDefault 单个工作流 input+output token 总量的默认上限；
	// 可用 CODE_WORKFLOW_MAX_TOKENS 环境变量或单次请求的 ?max_tokens= 覆盖。
	codeWorkflowMaxTokensDefault = 500000
	codeResultMaxChars           = 10000
	// codeRepeatCallLimit 同一 工具名+参数 在一个工作流里最多真实执行几次；
	// 动态结果工具另行豁免。普通工具留出合理的重试空间，超出才熔断。
	codeRepeatCallLimit = 4
)

// codeWorkflowTokenBudget 读取 token 预算：env 覆盖优先，否则用默认值。
func codeWorkflowTokenBudget() int {
	if v, err := strconv.Atoi(os.Getenv("CODE_WORKFLOW_MAX_TOKENS")); err == nil && v > 0 {
		return v
	}
	return codeWorkflowMaxTokensDefault
}

// codeWorkflowExhausted 判断是否该终止工作流：轮次或 token 预算任一触顶即真。
// 抽成纯函数是为了能脱离整个 SSE handler 单测（同 shouldBlockRepeat 的理由）。
func codeWorkflowExhausted(round, inputTokens, outputTokens, maxRounds, tokenBudget int) (bool, string) {
	if round >= maxRounds {
		return true, fmt.Sprintf("超过最大迭代轮数(%d)", maxRounds)
	}
	if inputTokens+outputTokens >= tokenBudget {
		return true, fmt.Sprintf("超过 token 预算(%d)", tokenBudget)
	}
	return false, ""
}

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
	// mode: yolo(全自动,默认) / ask(危险工具每步问)。
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

	// 生图提供商：前端设置面板选的，Go 侧拦截 image_generate 工具调用时自动注入，
	// 不走提示词——跟识图模型路由一个思路
	SetImageProvider(c.Query("image_provider"))

	// 允许单次请求覆盖轮次/token 预算（比如撞上限后带着更宽松的值 resume），
	// 不影响全局默认，也不持久化进检查点。
	maxRounds := codeWorkflowMaxRounds
	if v, err := strconv.Atoi(c.Query("max_rounds")); err == nil && v > 0 {
		maxRounds = v
	}
	tokenBudget := codeWorkflowTokenBudget()
	if v, err := strconv.Atoi(c.Query("max_tokens")); err == nil && v > 0 {
		tokenBudget = v
	}

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

	// steer 通道：同样按 workflowID 注册进全局 registry，供独立的
	// POST /api/code/workflow/steer 跨请求把消息塞进这条正在跑的循环。
	steerCh := registerSteerChannel(workflowID)
	defer unregisterSteerChannel(workflowID)

	// ask_user 提问等待通道：按 workflowID 注册，agent 调 ask_user 时阻塞在这里，
	// 等前端 POST /api/code/workflow/answer 唤醒。同一工作流串行（循环阻塞着），
	// 所以一个 channel 足够。
	askCh := registerAskUser(workflowID)
	defer unregisterAskUser(workflowID)

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
	fullHistory := r.chatHandler.sessionStore.Get(sessionID)
	history := truncateHistory(fullHistory, histLimit)
	msgs := provider.Invoking(history, task)

	// 上下文账本：记下"系统知道、但模型看不见"的事实（历史被截了多少、
	// 哪些工具输出被腰斩、压缩折叠了几轮），供 harness_status 自省用。
	ledger := newContextLedger()
	ledger.noteHistory(len(history), len(fullHistory), histLimit)
	// outcome 由各终止分支改写；用 defer 统一落盘，连"客户端断开直接 return"
	// 这种没有收尾事件的路径也能记上——那恰恰是最需要被统计到的一类中断。
	outcome := "interrupted"
	finalRound, finalIn, finalOut := 0, 0, 0
	defer func() {
		ledger.persist(ledgerRecord{
			WorkflowID: workflowID, SessionID: sessionID, Task: task,
			Outcome: outcome, Rounds: finalRound,
			InTokens: finalIn, OutTokens: finalOut,
			ActivatedTools: len(provider.ActivatedTools()),
		})
	}()
	// 归档目录按 TTL 淘汰，在这里扫一次即可（不再是"任务成功就删本次的全文"）
	go sweepToolOutputArchive()
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
	// previewOpened 自动预览只弹一次的哨兵（见下面 isFrontendEdit 那段）。
	// 不进检查点：续跑时重新弹一次预览是合理的——用户多半已经关掉页面了。
	previewOpened := false
	// currentTodos agent 自己维护的任务清单的权威副本。每轮重新注入上下文，
	// 免得它的计划被上下文压缩折叠掉之后只能靠回忆（见 todoContextLine）。
	var currentTodos []todoItem

	// 续跑：整体接管上面刚拼好的白板状态。msgs 用检查点里的完整对话
	// （含中断前所有 tool_calls 和工具结果），模型醒来就知道自己干到哪了。
	if resumed != nil {
		msgs = resumed.Msgs
		transcript = resumed.Transcript
		if resumed.CallSigCount != nil {
			callSignatureCount = resumed.CallSigCount
		}
		callSeq = resumed.CallSeq
		currentTodos = resumed.Todos // 续跑不丢主线
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
			Todos:          st.todos,
			InputTokens:    st.inputTokens, OutputTokens: st.outputTokens,
		})
	})
	// checkpoint 收拢 roundState 的组装，免得两个调用点各写一遍。
	checkpoint := func(round int) {
		provider.Invoked(round, roundState{
			msgs: msgs, transcript: transcript, callSigCount: callSignatureCount,
			callSeq: callSeq, inputTokens: inputTokens, outputTokens: outputTokens,
			todos: currentTodos,
		})
	}

	// 触发压缩用的上下文窗口：优先取模型实报的，取不到用兜底常量
	ctxWindow := estimatedContextWindow
	if len(backends) > 0 && backends[0].ContextWindow > 0 {
		ctxWindow = backends[0].ContextWindow
	}

	for round := startRound; ; round++ {
		// 轮次/token 预算任一触顶：跟错误路径一样保留检查点，可以带更宽松的
		// max_rounds/max_tokens resume 接着跑，而不是无条件判死刑。
		if done, reason := codeWorkflowExhausted(round, inputTokens, outputTokens, maxRounds, tokenBudget); done {
			outcome = "budget_exhausted"
			checkpoint(round)
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "failed", "final_output": reason + "，任务中止（可续跑）",
				"input_tokens": inputTokens, "output_tokens": outputTokens,
				"conversation_tokens": conversationTokens(inputTokens, staticSum),
				"resumable":           true, "workflow_id": workflowID,
			})
			return
		}
		if c.Request.Context().Err() != nil {
			return // 客户端断开
		}

		// 非阻塞取一条 steer 消息（如果有）：一轮最多消费一条，多条按发送顺序留在
		// channel 里排队到下一轮，避免把用户连续几句不同的话糊成一坨塞给模型。
		// 放在压缩之前，让插入的消息也参与后续的上下文预算核算。
		select {
		case steerMsg := <-steerCh:
			msgs = append(msgs, map[string]any{"role": "user", "content": "[用户中途插话] " + steerMsg})
			writeCodeSSE(c, "steering_injected", map[string]any{"message": steerMsg})
		default:
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
			// 记进账本：折叠掉的轮次原始细节已经不在上下文里了，模型自己看不出来，
			// 只有被告知才知道"这里有段记忆被换成摘要了"
			ledger.noteCompaction(compactionEvent{
				Round: round, FoldedMsgs: cr.FoldedMsgs,
				BeforeChars: cr.BeforeChars, AfterChars: cr.AfterChars,
			})
			log.Printf("🗜️ [压缩] workflow=%s 第 %d 轮：折叠 %d 条消息 %d→%d 字符",
				workflowID, round, cr.FoldedMsgs, cr.BeforeChars, cr.AfterChars)
		}

		// 任务清单每轮重新注入，且刻意放在压缩之后：压缩可能刚把携带计划的那条
		// assistant(tool_calls) 折叠成摘要，这里补回的是系统持有的权威副本。
		// 只挂在本轮请求上（roundMsgs），不写回 msgs——否则历史里会攒下一堆
		// 过时的清单快照，既费 token 又互相矛盾。
		roundMsgs := msgs
		if line := todoContextLine(currentTodos); line != "" {
			roundMsgs = append(append([]map[string]any{}, msgs...),
				map[string]any{"role": "user", "content": line})
		}

		finalRound = round // 账本落盘用（defer 里读的是最终值）
		content, calls, inTok, outTok, usedBackend, err := r.streamRouterRound(c, backends, roundMsgs, tools, effort, staticSum)
		// inTok 优先用上游真实 prompt_tokens；为 0 时退化为历史字符/4 估算（与四态机口径一致）
		if inTok > 0 {
			inputTokens = inTok
		}
		outputTokens += outTok
		finalIn, finalOut = inputTokens, outputTokens
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
			outcome = "upstream_error"
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
			outcome = "completed"
			deleteWorkflowCheckpoint(workflowID)
			// agent 决定结束对话：跑一次 build + 截图校验（旁路，失败不阻断）
			verifyOnWorkflowDone(c, workflowID)
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "completed", "final_output": content,
				"input_tokens": inputTokens, "output_tokens": outputTokens,
				"conversation_tokens": conversationTokens(inputTokens, staticSum),
			})
			if sessionID != "" {
				if content != "" {
					flowBlocks = append(flowBlocks, FlowBlock{Type: "intent", Text: content})
				}
				// 这里是全仓库唯一的历史落盘点，且只在工作流成功收尾时才走到，
				// 所以落进历史的每条任务按定义都已经结题——显式标出来，别让下一次
				// 对话把它读成"还没做的待办"（见 taskDone / buildChatMessages）。
				r.chatHandler.sessionStore.Append(sessionID, DSMessage{
					Role: "user", Content: task, Status: taskStatusCompleted,
				})
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
			// update_todo 是纯 UI 副作用（更新输入框上方的任务清单条），同样在这层办掉、不进执行链
			if tc.Function.Name == updateTodoToolName {
				items, ack := handleUpdateTodo(tc.Function.Arguments)
				handled[i] = ack
				if len(items) > 0 {
					currentTodos = items // 记成权威状态，每轮重新注入（见 todoContextLine）
					writeCodeSSE(c, "todo", map[string]any{"items": items})
				}
				continue
			}
			// read_skill 是纯查询（取技能库全文），没有 load_tools 那样的激活副作用
			if tc.Function.Name == readSkillToolName {
				handled[i] = handleReadSkill(tc.Function.Arguments, loadSkills())
				continue
			}
			// skill_manage 是显式 /learn 的候选箱；不进普通执行链，也不自动启用。
			if tc.Function.Name == skillManageToolName {
				handled[i] = handleSkillManage(tc.Function.Arguments)
				continue
			}
			// harness_status 是纯自省：读账本，不碰外部世界
			if tc.Function.Name == harnessStatusToolName {
				handled[i] = handleHarnessStatus(tc.Function.Arguments, ledger, round,
					contextBreakdown, activatedToolNames(provider.ActivatedTools()))
				continue
			}
			// ask_user：让 agent 在跑一半时向用户提问并暂停。本层直接办掉——
			// 推 question 事件 + 阻塞等回答 + 把答案作为工具结果注入上下文，
			// 不进 executeCodeCalls（它不碰 MCP/外部命令）。返回的答案既填进
			// tool 消息，也落进可视化轨迹 FlowBlock（带问/答，刷新后仍可见）。
			if tc.Function.Name == askUserToolName {
				ans, qb := handleAskUser(c, workflowID, askCh, tc.Function.Arguments)
				handled[i] = "用户回答：" + ans
				flowBlocks = append(flowBlocks, qb)
				continue
			}
			// capture_preview / open_preview 是 harness 内置常驻工具，在下方 564 的
			// switch 里直接处理（设 results[i]），不进 executeCodeCalls——否则会因非
			// mcp__ 前缀被 872 行误判为「未知工具」。提前 continue 掉，避免被塞进 toRun。
			if tc.Function.Name == "capture_preview" || tc.Function.Name == "open_preview" ||
				tc.Function.Name == "inject_preview_js" {
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
					output: fmt.Sprintf("已阻止：%s 用完全相同的参数已连续执行 %d 次。为避免工具调用陷入循环，本次未再执行；请复用已有结果或调整参数。",
						calls[i].Function.Name, codeRepeatCallLimit),
				}
			case handled[i] != "":
				results[i] = codeExecResult{output: handled[i]}
			case calls[i].Function.Name == "capture_preview":
				// harness 截「用户正在看的内嵌预览页」并作为图片工件发聊天。
				png, cerr := capturePreviewScreenshot(extractURLArg(calls[i].Function.Arguments))
				if cerr != nil {
					results[i] = codeExecResult{failed: true, output: "截图失败：" + cerr.Error()}
				} else {
					output := "已截取内嵌预览页面（见下方图片工件）。"
					// 对分数、计数器等状态问题，优先把同一 live target 的 DOM 文本
					// 一并给 agent；不能让小字号截图成为唯一事实来源。
					if text, terr := readCurrentPreviewText(); terr == nil && text != "" {
						output += "\n【同一预览页面的 DOM 文本（状态判断优先依据此处）】\n" + text
					}
					results[i] = codeExecResult{
						output: output,
						images: []mcpImageArtifact{{Data: base64.StdEncoding.EncodeToString(png), MimeType: "image/png"}},
					}
				}
			case calls[i].Function.Name == "open_preview":
				// agent 主动把指定页面弹进内嵌预览面板（harness CDP 通道）。
				// 必须补发 preview_open SSE —— 前端 PreviewBrowser 只认这个事件去连 CDP
				// 渲染（cdp_ws 非空走 startScreencast 真实可交互渲染，否则降级 iframe）。
				// 只回文本不给事件的话，后端开了页面但前端不会弹（之前的 bug）。
				addr, cdpWS, perr := openPreviewInPanel(calls[i].Function.Arguments)
				if perr != nil {
					results[i] = codeExecResult{failed: true, output: "打开预览失败：" + perr.Error()}
				} else {
					writeCodeSSE(c, "preview_open", map[string]any{"url": addr, "cdp_ws": cdpWS})
					results[i] = codeExecResult{output: "已把页面弹进内嵌预览面板，用户现在可以直接查看并交互：" + addr}
				}
			case calls[i].Function.Name == "inject_preview_js":
				// 前端设计 Agent 在用户正在看的预览页中执行通用检查或交互脚本。
				var a struct {
					JS string `json:"js"`
				}
				if err := json.Unmarshal([]byte(calls[i].Function.Arguments), &a); err != nil || strings.TrimSpace(a.JS) == "" {
					results[i] = codeExecResult{failed: true, output: "inject_preview_js 需要 js 参数（要注入的 JS 字符串）"}
					break
				}
				msg, ierr := injectPreviewJS(a.JS)
				if ierr != nil {
					results[i] = codeExecResult{failed: true, output: "注入失败：" + ierr.Error()}
				} else {
					results[i] = codeExecResult{output: msg}
				}
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
			out, archived := compactToolOutput(workflowID, tc.ID, tc.Function.Name, results[i].output)
			ledger.noteArchive(archived)
			writeCodeSSE(c, "result", map[string]any{
				"id": tc.ID, "name": tc.Function.Name, "ok": !results[i].failed, "output": full,
			})
			// 图像是 MCP 工具的通用返回工件：Agent 无论在何种任务里主动截图，
			// 都会自动作为一条聊天交付消息出现，不依赖某个写死的工作流分支或 UI 按钮。
			for imageIndex, image := range results[i].images {
				mime := image.MimeType
				if mime == "" {
					mime = "image/png"
				}
				writeCodeSSE(c, "artifact", map[string]any{
					"id":         fmt.Sprintf("%s_image_%d", tc.ID, imageIndex),
					"kind":       "image",
					"tool":       tc.Function.Name,
					"image":      "data:" + mime + ";base64," + image.Data,
					"source_url": artifactSourceURL(tc.Function.Arguments),
					"caption":    "Agent 已截取当前页面，作为本次交付凭证。",
				})
			}
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

			// 前端文件被改动 → 自动把预览面板弹出来。整个工作流只弹一次：
			// 一次任务改十个文件不该弹十次，用户手动关掉后也不该被反复强开。
			if !previewOpened && !results[i].failed && isFrontendEdit(tc.Function.Name, tc.Function.Arguments) {
				previewOpened = true

				// 优先用 CDP 在真实 Chromium 里渲染 agent 刚改的那个文件，把 target
				// 的 ws 回给前端做 screencast（不再 iframe 整站）。HTML 的 CDP 失败会
				// 显式推错误；只有非 HTML 文件才降级为前端 dev server 首页（iframe）。
				var editPath string
				if p, e := parseFrontendEditPath(tc.Function.Arguments); e == nil {
					editPath = p
				}
				url, cdpWS, cdpError, ok := autoOpenBrowserPreview(editPath)
				if ok {
					writeCodeSSE(c, "preview_open", map[string]any{"url": url, "cdp_ws": cdpWS})
					log.Printf("🖥️ [预览] workflow=%s CDP 真实渲染 %s", workflowID, url)
				} else if cdpError != "" {
					writeCodeSSE(c, "preview_open", map[string]any{"url": url, "cdp_error": cdpError})
					log.Printf("🖥️ [预览] workflow=%s %s", workflowID, cdpError)
				} else if url != "" {
					writeCodeSSE(c, "preview_open", map[string]any{"url": url})
					log.Printf("🖥️ [预览] workflow=%s 降级打开首页 %s", workflowID, url)
				}
			}
		}
		inputTokens += len(content) / 4

		// 落盘断点：此刻这一轮的工具全部执行完、结果已进 msgs，没有半途状态，
		// 是唯一安全的恢复边界。后端从这里被杀掉，续跑就从下一轮问模型开始。
		checkpoint(round + 1)

		// 整轮调用全被熔断 → 模型已经在原地打转，提示也没拉回来，直接收尾，
		// 而不是陪它空转到 codeWorkflowMaxRounds。
		if allBlocked {
			outcome = "repeat_blocked"
			deleteWorkflowCheckpoint(workflowID)
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "failed", "final_output": "检测到模型重复调用同一工具且无进展，已中止任务。",
				"input_tokens": inputTokens, "output_tokens": outputTokens,
				"conversation_tokens": conversationTokens(inputTokens, staticSum),
			})
			return
		}
	}
}

// artifactSourceURL 尽力从工具参数里取出展示用来源地址。取不到不影响工件发布；
// browser_snapshot 这类复用已有会话的调用本来就没有 url 参数。
func artifactSourceURL(argsJSON string) string {
	var args struct {
		URL string `json:"url"`
	}
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return ""
	}
	return args.URL
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
	images []mcpImageArtifact
}

// frontendEditTools 会真正改动文件内容的 MCP 文件工具。读类工具不算——
// 光看一眼文件不该弹预览。
var frontendEditTools = map[string]bool{
	"write_file":           true,
	"edit_file":            true,
	"mcp__fs__write_file":  true,
	"mcp__fs__edit_file":   true,
	"mcp__fs__create_file": true,
}

// frontendExts 命中即认为这次改动会影响浏览器里的呈现。
var frontendExts = []string{
	".vue", ".jsx", ".tsx", ".svelte", ".html", ".css", ".scss", ".less", ".ts", ".js",
}

// isFrontendEdit 判断一次工具调用是不是"改了前端文件"，用来决定要不要自动弹预览。
// 抽成纯函数是为了能脱离 SSE handler 单测（同 shouldBlockRepeat / codeWorkflowExhausted）。
// 参数解析失败一律返回 false：宁可不弹，也不能因为解析问题让主流程出岔子。
func isFrontendEdit(toolName, argsJSON string) bool {
	if !frontendEditTools[toolName] {
		return false
	}
	var args struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return false
	}
	p := strings.ToLower(args.Path)
	for _, ext := range frontendExts {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// repeatGuardExemptTools 的结果依赖当前页面或操作时机：即使参数完全相同，下一次
// 截图、重新打开或再次注入也可能得到不同结果，不能套用静态调用的重复熔断。
var repeatGuardExemptTools = map[string]bool{
	"capture_preview":   true,
	"open_preview":      true,
	"inject_preview_js": true,
}

// shouldBlockRepeat 只统计连续的同签名调用：中间执行过其他工具就重置。页面或文件
// 状态可能已被其他操作改变，不能把整轮工作流里相隔很远的同参数调用累计成死循环。
// 同一签名连续真实执行次数达到 limit 后，第 limit+1 次及以后返回 true。
// 抽成纯函数是为了能脱离整个 SSE handler 单测——熔断是"模型抽风时的护栏"，
// 健康模型看得见工具结果就不会重复调，实况里几乎不触发，只能靠单测覆盖。
func shouldBlockRepeat(counts map[string]int, name, args string, limit int) bool {
	if repeatGuardExemptTools[name] {
		clear(counts)
		return false
	}
	sig := name + "|" + args
	if _, continuing := counts[sig]; !continuing {
		clear(counts)
	}
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

	// maybeRequestApproval 在 ask 模式下发起审批拦截；返回是否允许执行。
	// 两类要批：危险工具（写盘/执行命令），以及碰了工作目录之外路径的任何工具
	// ——后者以前是 MCP 层直接硬报错，agent 只能把文件都往工作目录里塞。
	// yolo 模式 / 非危险且未越界 / 已设 don't-ask-again → 直接放行。
	maybeRequestApproval := func(tc core.ToolCall) bool {
		name := tc.Function.Name
		if mode == "yolo" {
			// Yolo 畅通无阻：危险工具与越界访问一律不拦——但不可逆文件操作
			// （删除/移动/重命名）除外，必须进下方审批，避免 agent 全自动误删。
			if !isIrreversibleTool(name) {
				return true
			}
		}
		outside, outPath := toolOutsideRoot(tc.Function.Arguments)
		// Harness 的统一只读判定：无论哪个 Agent、哪个前端入口，只要参数
		// 本身是安全读取，就不弹审批；不是由某个 Git Agent 特例决定的。
		if !outside && isReadOnlyToolCall(name, tc.Function.Arguments) {
			return true
		}
		if !isDangerousTool(name) && !outside {
			return true
		}
		// 越界的 don't-ask-again 按目录记，普通危险工具按工具名记（粒度见 approval.go）
		key := rememberKey(name)
		if outside {
			key = outsideRememberKey(outPath)
		}
		if r.shouldAutoApproveKey(sessionID, key) {
			return true
		}
		// 登记 + 推 SSE 事件 + 阻塞等批准。approval id 编码 workflowID::callID，
		// 让独立的 approve 端点能反解出 waiter。
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("approval_%d", time.Now().UnixNano())
		}
		approvalID := workflowID + "::" + id
		waiter.expect(approvalID, key)
		payload := map[string]any{
			"id":   approvalID,
			"tool": name,
			"args": tc.Function.Arguments,
			"mode": mode,
		}
		if outside {
			// 前端据此把批准条文案换成「这个路径在工作目录之外」，而不是笼统的危险工具
			payload["reason"] = "path_outside_workdir"
			payload["path"] = outPath
			payload["workdir"] = core.GetProjectRoot()
		}
		writeCodeSSE(c, "approval_request", payload)
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
		if isNativeOnDemandTool(name) {
			var args map[string]any
			if err := json.Unmarshal([]byte(defaultJSONObject(tc.Function.Arguments)), &args); err != nil {
				results[i] = codeExecResult{output: "内置工具参数失败: " + err.Error(), failed: true}
				continue
			}
			var preEditLine int
			if name == "edit_file" {
				preEditLine = r.calcEditStartLine(tc.Function.Arguments)
			}
			if name == "write_file" || name == "edit_file" {
				OnBeforeWrite(name, args)
			}
			nativeResult, err := callNativeTool(c.Request.Context(), name, tc.Function.Arguments)
			if err != nil {
				results[i] = codeExecResult{output: "内置工具失败: " + err.Error(), failed: true}
			} else {
				if name == "write_file" || name == "edit_file" {
					OnAfterWrite(name, args)
				}
				out := nativeResult.Text
				if name == "edit_file" && preEditLine > 0 && !strings.Contains(out, "第") {
					out = fmt.Sprintf("%s（第 %d 行）", out, preEditLine)
				}
				results[i] = codeExecResult{output: out, images: nativeResult.Images}
			}
			continue
		}
		if strings.HasPrefix(name, "mcp__") {
			// MCP edit_file：执行前读文件记下行号，执行后补到结果里
			var preEditLine int
			if name == "mcp__fs__edit_file" {
				preEditLine = r.calcEditStartLine(tc.Function.Arguments)
			}
			mcpResult, err := callMCPToolWithArtifacts(name, tc.Function.Arguments)
			if err != nil {
				results[i] = codeExecResult{output: "MCP 工具失败: " + err.Error(), failed: true}
			} else {
				out := mcpResult.Text
				if name == "mcp__fs__edit_file" && preEditLine > 0 && !strings.Contains(out, "第") {
					out = fmt.Sprintf("%s（第 %d 行）", out, preEditLine)
				}
				results[i] = codeExecResult{output: out, images: mcpResult.Images}
			}
			continue
		}
		// 到这里说明是个既非 Go 内置/MCP、也非编排类的工具名。
		results[i] = codeExecResult{
			output: fmt.Sprintf("未知工具 %s：请对照按需工具索引，先用 load_tools 加载再调用。", name),
			failed: true,
		}
	}

	wg.Wait()
	return results
}

// buildCodeWorkflowTools 已挪到 tool_ondemand.go：MCP 工具改为按需加载，
// 不再无条件全量塞进每一轮请求（实测省 6000+ tok/轮）。
// search_memory 内置工具已随 core.ExecuteToolCall 一并退役：SwiftNet 的无条件记忆
// 注入已把身份/工作态/收件箱塞进每轮系统提示词，显式再搜一遍实测收益甚微。

// calcEditStartLine 在 MCP edit_file 执行前读文件，计算 oldText 的起始行号。
func (r *WorkflowRunner) calcEditStartLine(argsJSON string) int {
	var args struct {
		Path  string `json:"path"`
		Edits []struct {
			OldText string `json:"oldText"`
		} `json:"edits"`
		OldText string `json:"oldText"`
		OldStr  string `json:"old_string"` // 模型偶尔把这个工具当内置 edit_file 的扁平 schema 调，见 normalizeMCPEditArgs
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
