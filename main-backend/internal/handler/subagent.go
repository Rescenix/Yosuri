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

// 子代理可用的 Go 内置只读工具。基础调研不再要求 Python/npm 子进程。
var subAgentNativeTools = map[string]bool{
	"read_file":      true,
	"grep":           true,
	"glob":           true,
	"list_directory": true,
	"directory_tree": true,
	"get_file_info":  true,
	"web_fetch":      true,
	"web_search":     true,
	"view_image":     true,
	"memory_search":  true,
	"workdir_read":   true,
}

// 子代理可用的 MCP server 白名单：给用户主动配置的外部扩展保留兼容。
// fs 不在这里——它有写删类工具，单独在 isSubagentMCPToolAllowed 里按只读过滤。
var subAgentMCPServers = map[string]bool{
	"grep":       true, // grep 全文检索 + read_range 按行读
	"web_fetch":  true, // 抓网页
	"view_image": true, // 看图
}

// isSubagentMCPToolAllowed 判定一个 mcp__<server>__<tool> 是否放行给子代理。
// 子代理是只读调研代理，读文件/列目录也统一走 MCP（fs 的只读子集），
// 不再依赖任何内置工具——core.ChatTools 的文件工具已随之退役。
func isSubagentMCPToolAllowed(name string) bool {
	if !strings.HasPrefix(name, "mcp__") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(name, "mcp__"), "__", 2)
	if len(parts) != 2 {
		return false
	}
	server := parts[0]
	// fs 只放只读工具（read/list/get/directory_tree），写删类由 isDangerousTool 挡掉
	if server == "fs" {
		return !isDangerousTool(name)
	}
	return subAgentMCPServers[server]
}

func subAgentToolsWire() []map[string]any {
	var out []map[string]any
	for _, t := range allOnDemandToolDefs() {
		if !subAgentNativeTools[t.Function.Name] && !isSubagentMCPToolAllowed(t.Function.Name) {
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
你只有只读工具：读文件用 read_file，列目录用 list_directory，全文检索用 grep。
不要尝试修改任何文件。工作目录是 %s。`, core.GetProjectRoot())},
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
			if subAgentNativeTools[tc.Function.Name] {
				if res, err := callNativeTool(ctx, tc.Function.Name, tc.Function.Arguments); err != nil {
					out = "工具执行失败: " + err.Error()
				} else {
					out = res.Text
				}
			} else if isSubagentMCPToolAllowed(tc.Function.Name) {
				if res, err := callMCPTool(tc.Function.Name, tc.Function.Arguments); err != nil {
					out = "工具执行失败: " + err.Error()
				} else {
					out = res
				}
			} else {
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
