package handler

// 雨燕子代理 —— 仿 Hermes 子代理系统。
//
// 主 Agent 通过 dispatch_agent 工具派发只读调研子任务（读代码/搜索/分析），
// 一轮内的多个 dispatch_agent 调用由 executeCodeCalls 并行执行（雨燕群）。
// 子代理走非流式 DS 调用（结果只回给主 Agent，不需要打字机效果），
// 工具集锁死为只读白名单，天然无法越权改文件。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"backend/internal/ai/core"
)

const subAgentMaxRounds = 6
const subAgentResultMaxChars = 8000

const subAgentUsagePrompt = `
━━━ 子代理（雨燕） ━━━
遇到需要大量阅读/检索的复杂任务，可用 dispatch_agent 把独立的只读调研子任务
（读代码、搜索、分析结构、抓取网页、看图）派发给子代理，一轮内多个 dispatch_agent 会并行执行。
子代理只有只读工具（含 grep 全文检索、web_fetch 抓网页、view_image 看图），无法修改文件——
所有修改类操作必须由你自己完成。简单任务不要派发子代理，直接做，避免浪费 token。
`

var dispatchAgentToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "dispatch_agent",
		Description: "派发一个只读调研子代理（雨燕）去独立完成子任务：读代码、搜索代码库、分析结构等。子代理无法修改文件。一轮内多个 dispatch_agent 调用会并行执行，适合把大调研拆成几块同时跑。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"task": {
					Type:        "string",
					Description: "子任务描述，要自包含：子代理看不到你和用户的对话，把必要的背景写进来",
				},
				"context": {
					Type:        "string",
					Description: "可选，补充上下文（相关文件路径、已知结论等）",
				},
			},
			Required: []string{"task"},
		},
	},
}

// 子代理只读工具白名单（内置工具）
var subAgentToolNames = map[string]bool{
	"read_file":     true,
	"list_dir":      true, // 调研任务几乎都从看目录结构开始，没有它统计类任务必然不收敛
	"search_memory": true,
}

// 子代理可用的 MCP 工具：按「server 名」白名单放行整个 server，而不是逐个工具名——
// grep/glob 都在 grep server 下、只读；web_fetch/view_image 也天然只读、无副作用。
// 不放行 fs（写删类）和 generate_image（有副作用/耗时），子代理管不了这些。
var subAgentMCPServers = map[string]bool{
	"grep":       true,
	"web_fetch":  true,
	"view_image": true,
}

// isSubagentMCPToolAllowed 判定一个 mcp__<server>__<tool> 形式的工具名是否放行给子代理。
func isSubagentMCPToolAllowed(name string) bool {
	if !strings.HasPrefix(name, "mcp__") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(name, "mcp__"), "__", 2)
	return len(parts) == 2 && subAgentMCPServers[parts[0]]
}

func subAgentToolsWire() []map[string]any {
	var out []map[string]any
	for _, t := range core.ChatTools {
		if !subAgentToolNames[t.Function.Name] {
			continue
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			},
		})
	}
	// MCP 工具懒加载，第一次调用时才真正拉起各 server 子进程；idempotent，
	// 主 Agent 那边通常已经初始化过，这里只是确保子代理独立运行时也不会拿到空表。
	for _, t := range loadMCPToolDefs() {
		if !isSubagentMCPToolAllowed(t.Function.Name) {
			continue
		}
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

// runSubAgent 跑一个完整的子代理循环，返回其最终结论文本。
// 走完整模型路由链（与主 Agent 同一条链，失败秒切）。
// id 用主 Agent 的 tool_call ID，前端据此把生命周期事件挂到对应的后台任务卡片；
// emit 把 subagent_start/progress/done 实时写进 SSE 流（可为 nil）。
func runSubAgent(ctx context.Context, backends []RouterBackend, id, argsJSON string, emit func(string, map[string]any)) (string, error) {
	if emit == nil {
		emit = func(string, map[string]any) {}
	}
	var args struct {
		Task    string `json:"task"`
		Context string `json:"context"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Task == "" {
		return "", fmt.Errorf("dispatch_agent 需要 task 参数")
	}

	emit("subagent_start", map[string]any{"id": id, "task": args.Task})

	userMsg := args.Task
	if args.Context != "" {
		userMsg += "\n\n补充上下文：\n" + args.Context
	}

	msgs := []map[string]any{
		{"role": "system", "content": fmt.Sprintf(`你是雨燕子代理，负责独立完成一个只读调研子任务。
用最少的工具调用拿到答案，然后输出简明结论（要点式，不要铺陈）——你的输出会直接回给主 Agent 当调研结果用，token 是成本。
你只有只读工具，不要尝试修改任何文件。工作目录是 %s。`, core.GetProjectRoot())},
		{"role": "user", "content": userMsg},
	}
	tools := subAgentToolsWire()

	for round := 0; round < subAgentMaxRounds; round++ {
		if ctx.Err() != nil {
			emit("subagent_done", map[string]any{"id": id, "ok": false, "rounds": round, "output": "已取消"})
			return "", ctx.Err()
		}
		content, calls, err := routeChatOnce(ctx, backends, msgs, tools)
		if err != nil {
			emit("subagent_done", map[string]any{"id": id, "ok": false, "rounds": round, "output": err.Error()})
			return "", err
		}
		if len(calls) == 0 {
			emit("subagent_done", map[string]any{"id": id, "ok": true, "rounds": round, "output": truncateChars(content, 500)})
			return content, nil
		}
		for _, tc := range calls {
			emit("subagent_progress", map[string]any{
				"id": id, "round": round, "tool": tc.Function.Name,
				"args_preview": truncateChars(tc.Function.Arguments, 120),
			})
		}

		var dsCalls []map[string]any
		for i := range calls {
			if calls[i].ID == "" {
				calls[i].ID = fmt.Sprintf("sub_call_%d_%d", round, i)
			}
			dsCalls = append(dsCalls, map[string]any{
				"id": calls[i].ID, "type": "function",
				"function": map[string]any{"name": calls[i].Function.Name, "arguments": calls[i].Function.Arguments},
			})
		}
		msgs = append(msgs, map[string]any{"role": "assistant", "content": content, "tool_calls": dsCalls})

		for _, tc := range calls {
			var out string
			switch {
			case subAgentToolNames[tc.Function.Name]:
				if res, err := core.ExecuteToolCall(tc); err != nil {
					out = "工具执行失败: " + err.Error()
				} else {
					out = res.Content
				}
			case isSubagentMCPToolAllowed(tc.Function.Name):
				if res, err := callMCPTool(tc.Function.Name, tc.Function.Arguments); err != nil {
					out = "工具执行失败: " + err.Error()
				} else {
					out = res
				}
			default:
				out = fmt.Sprintf("工具 %s 对子代理不可用（只读白名单）", tc.Function.Name)
			}
			msgs = append(msgs, map[string]any{
				"role": "tool", "tool_call_id": tc.ID,
				"content": truncateChars(out, subAgentResultMaxChars),
			})
		}
	}
	emit("subagent_done", map[string]any{"id": id, "ok": false, "rounds": subAgentMaxRounds, "output": "超过最大轮数未收敛"})
	return "", fmt.Errorf("子代理超过最大轮数(%d)未收敛", subAgentMaxRounds)
}

// 非流式单次调用已泛化为 model_router.go 的 openAIChatOnce / routeChatOnce。
