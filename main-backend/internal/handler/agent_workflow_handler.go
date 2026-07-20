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
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"backend/internal/agent"
	"backend/internal/ai/core"
	"backend/internal/swiftnet"

	"github.com/gin-gonic/gin"
)

const (
	codeWorkflowMaxRounds = 20
	codeResultMaxChars    = 10000
)

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
	task := strings.TrimSpace(c.Query("task"))
	if task == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task 参数必填"})
		return
	}
	sessionID := c.Query("session_id")
	// mode: yolo(全自动,默认) / ask(危险工具每步问)。不传或非法值(含旧 plan)按 yolo 处理。
	mode := strings.ToLower(c.Query("mode"))
	if mode != "ask" {
		mode = "yolo"
	}

	// 模型路由链：前端选了具体模型就精确路由到那一个；否则走用户配置>env DeepSeek>
	// 免费池>本地兜底的全链（本地恒在，链永不为空）
	backends := resolveBackends(c.Query("openid"), c.Query("model"))
	effort := c.Query("effort") // "low"/"medium"/"high"，只有 backend.Reasoning=true 时才真的生效

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	workflowID := agent.NewWorkflowID()
	writeCodeSSE(c, "workflow_start", map[string]any{"workflow_id": workflowID, "task": task, "mode": mode})

	// 审批等待器：整个工作流生命周期共用一个（channel 按 approval id 区分，不会跨轮串）。
	// 注册进全局 registry，供独立的 POST /api/code/workflow/approve 跨请求唤醒。
	waiter := newApprovalWaiter()
	registerApprovalWaiter(workflowID, waiter)
	defer unregisterApprovalWaiter(workflowID)

	// SwiftNet 三区里 pinned(身份)/handoff(工作态)/inbox 按设计就该"无条件注入"——
	// 预算压在 <500 tok 就是为了能无条件塞，不用等模型自己想起来调 search_memory 才有。
	// 之前只有 /api/chat/stream 的纯聊天路径接了这个（还额外加了个 LLM 判定门槛，
	// 违背了"无条件"的本意），四态机这条真正在跑 code 任务的主路径完全没接，
	// agent 每次开工都是失忆状态，只能靠自己想起来查 search_memory 兜底。
	systemBase := agent.MainAgentConfigNative().SystemPrompt
	skillPrompt := skillLibraryPrompt()
	systemPrompt := systemBase + subAgentUsagePrompt + skillPrompt
	memoryInject := swiftnet.Default().UnconditionalInject()
	if memoryInject != "" {
		systemPrompt += "\n\n# 长期记忆（无条件注入，身份/工作态/收件箱）\n" + memoryInject
	}
	// 用户在「我的」tab 填的称呼/职业/自定义指令，必须注入主链路系统提示词。
	// 之前只挂在废弃的 /api/chat/stream（buildSystemPrompt）里，四态机收不到——
	// 清理旧路径时这行若不先落过来，自定义指令功能会随 chat_stream 一起被删没。
	systemPrompt += userInstructionsPrompt()
	tools := buildCodeWorkflowTools()
	// 分类上下文占用（token 估算口径与四态机一致：字符数/4），随 model_info 回传前端展示。
	toolsJSON, _ := json.Marshal(tools)
	contextBreakdown := map[string]int{
		"system":  estimateTokenCount(systemBase),
		"subagent": estimateTokenCount(subAgentUsagePrompt),
		"skill":    estimateTokenCount(skillPrompt),
		"memory":   estimateTokenCount(memoryInject),
		"tools":    estimateTokenCount(string(toolsJSON)),
	}

	// 之前这里每次都是只有 system+当前 task 的白板，session_id 传了但从没读过——
	// LLM 完全不知道上一条消息说了什么。跟 chat_stream 那条老路径一样，从
	// sessionStore 捞这个会话的历史（截断到最近 maxHistoryMessages 条）拼进去，
	// 工作流结束后再把这一轮的 user/assistant 写回去，下一条消息才能接上下文。
	history := r.chatHandler.sessionStore.Get(sessionID)
	history = truncateHistory(history, maxHistoryMessages)
	built := buildChatMessages(systemPrompt, history, task)
	msgs := make([]map[string]any, len(built))
	historyChars := 0
	for i, m := range built {
		msgs[i] = map[string]any{"role": m["role"], "content": m["content"]}
		historyChars += len(m["content"])
	}

	var transcript []string // 动作摘要，供技能生成
	inputTokens := historyChars / 4
	outputTokens := 0
	callSeq := 0
	modelInfoSent := false

	for round := 0; round < codeWorkflowMaxRounds; round++ {
		if c.Request.Context().Err() != nil {
			return // 客户端断开
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
			writeCodeSSE(c, "flow_error", map[string]any{"message": err.Error()})
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "failed", "final_output": "任务执行失败: " + err.Error(),
				"input_tokens": inputTokens, "output_tokens": outputTokens,
			})
			return
		}

		// 没有工具调用 → 最终回答，收尾
		if len(calls) == 0 {
			writeCodeSSE(c, "workflow_done", map[string]any{
				"status": "completed", "final_output": content,
				"input_tokens": inputTokens, "output_tokens": outputTokens,
			})
			if sessionID != "" {
				r.chatHandler.sessionStore.Append(sessionID, DSMessage{Role: "user", Content: task})
				r.chatHandler.sessionStore.Append(sessionID, DSMessage{Role: "assistant", Content: content})
			}
			go generateSkillAsync(task, transcript)
			return
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
		// 审批等待器：仅 ask 模式需要；yolo 模式下 executeCodeCalls 内部会直接跳过拦截。
		// 使用整个 workflow 共用的 waiter（按 approval id 区分），不每轮新建。
		results := r.executeCodeCalls(c, backends, calls, emit, mode, waiter, sessionID, workflowID)

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
			out := truncateChars(results[i].output, codeResultMaxChars)
			writeCodeSSE(c, "result", map[string]any{
				"id": tc.ID, "name": tc.Function.Name, "ok": !results[i].failed, "output": out,
			})
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": tc.ID, "content": out})
			inputTokens += len(out) / 4
			transcript = append(transcript, fmt.Sprintf("%s(%s) => %s",
				tc.Function.Name, truncateChars(tc.Function.Arguments, 300), truncateChars(results[i].output, 200)))
		}
		inputTokens += len(content) / 4
	}

	writeCodeSSE(c, "workflow_done", map[string]any{
		"status": "failed", "final_output": fmt.Sprintf("超过最大迭代轮数(%d)，任务中止", codeWorkflowMaxRounds),
		"input_tokens": inputTokens, "output_tokens": outputTokens,
	})
}

type codeExecResult struct {
	output string
	failed bool
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
			out, err := callMCPTool(name, tc.Function.Arguments)
			if err != nil {
				results[i] = codeExecResult{output: "MCP 工具失败: " + err.Error(), failed: true}
			} else {
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

// buildCodeWorkflowTools 组装本工作流可用的完整工具集：
// 内置语义/记忆工具 + dispatch_agent + MCP 生态工具，序列化成 DS 的 tools 参数格式。
// 内置文件类工具（read/write/edit/list_dir/execute_command）已由 MCP filesystem
// 取代，故在此过滤掉，只让 MCP 提供文件能力。
func buildCodeWorkflowTools() []map[string]any {
	defs := make([]core.ToolDefinition, 0, len(core.ChatTools)+8)
	for _, t := range core.ChatTools {
		if hardcodeFileTools[t.Function.Name] {
			continue
		}
		defs = append(defs, t)
	}
	defs = append(defs, dispatchAgentToolDef)
	defs = append(defs, loadMCPToolDefs()...)

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
