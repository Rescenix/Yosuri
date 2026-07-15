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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

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

	// 模型路由链：前端选了具体模型就精确路由到那一个；否则走用户配置>env DeepSeek>
	// 免费池>本地兜底的全链（本地恒在，链永不为空）
	backends := resolveBackends(c.Query("openid"), c.Query("model"))
	effort := c.Query("effort") // "low"/"medium"/"high"，只有 backend.Reasoning=true 时才真的生效

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	workflowID := agent.NewWorkflowID()
	writeCodeSSE(c, "workflow_start", map[string]any{"workflow_id": workflowID, "task": task})

	// SwiftNet 三区里 pinned(身份)/handoff(工作态)/inbox 按设计就该"无条件注入"——
	// 预算压在 <500 tok 就是为了能无条件塞，不用等模型自己想起来调 search_memory 才有。
	// 之前只有 /api/chat/stream 的纯聊天路径接了这个（还额外加了个 LLM 判定门槛，
	// 违背了"无条件"的本意），四态机这条真正在跑 code 任务的主路径完全没接，
	// agent 每次开工都是失忆状态，只能靠自己想起来查 search_memory 兜底。
	systemPrompt := agent.MainAgentConfigNative().SystemPrompt + subAgentUsagePrompt + skillLibraryPrompt()
	if base := swiftnet.Default().UnconditionalInject(); base != "" {
		systemPrompt += "\n\n# 长期记忆（无条件注入，身份/工作态/收件箱）\n" + base
	}
	tools := buildCodeWorkflowTools()

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

		content, calls, outTok, usedBackend, err := r.streamRouterRound(c, backends, msgs, tools, effort)
		outputTokens += outTok
		// 只在第一轮实际承接请求后发一次——同一个工作流后续轮次不会换 backend，
		// 前端只需要知道"这次对话用的是哪个模型、它能不能识图/支持多大上下文"一次就够
		if usedBackend != nil && !modelInfoSent {
			modelInfoSent = true
			writeCodeSSE(c, "model_info", map[string]any{
				"name": usedBackend.Name, "vision": usedBackend.Vision,
				"context_window": usedBackend.ContextWindow, "reasoning": usedBackend.Reasoning,
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
		results := r.executeCodeCalls(c.Request.Context(), backends, calls, emit)

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
func (r *WorkflowRunner) executeCodeCalls(ctx context.Context, backends []RouterBackend, calls []core.ToolCall, emit func(string, map[string]any)) []codeExecResult {
	results := make([]codeExecResult, len(calls))
	var wg sync.WaitGroup

	for i, tc := range calls {
		if tc.Function.Name == "dispatch_agent" {
			wg.Add(1)
			go func(i int, tc core.ToolCall) {
				defer wg.Done()
				out, err := runSubAgent(ctx, backends, tc.ID, tc.Function.Arguments, emit)
				if err != nil {
					results[i] = codeExecResult{output: "子代理执行失败: " + err.Error(), failed: true}
					return
				}
				results[i] = codeExecResult{output: out}
			}(i, tc)
		}
	}

	for i, tc := range calls {
		name := tc.Function.Name
		if name == "dispatch_agent" {
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

// buildCodeWorkflowTools 组装本工作流可用的完整工具集：
// 内置工具 + dispatch_agent + MCP 生态工具，序列化成 DS 的 tools 参数格式。
func buildCodeWorkflowTools() []map[string]any {
	defs := make([]core.ToolDefinition, 0, len(core.ChatTools)+8)
	defs = append(defs, core.ChatTools...)
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
