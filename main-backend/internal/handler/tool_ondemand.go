package handler

// 工具按需加载 —— 默认只把 MCP 工具的「名字 + 一句话」放进上下文，
// 模型要用哪个再调 load_tools 取回完整 schema，之后该工具才进 tools 数组。
//
// 为什么：实测（context_budget_test.go）全量 schema 占 6563 tok，是整个静态前缀的 91%，
// 而且每轮都要重发一遍——20 轮就是 13 万 token 只为反复描述工具长什么样。
// 一个任务真正会用到的通常是 2-4 个工具，剩下 25 个的 schema 是纯粹的浪费。
//
// 为什么不用「万能代理工具」（call_mcp_tool(name, args)）：那样模型是照着一句话描述
// 猜参数，参数名写错了才在执行时报错。这里保持原生 function calling——工具一旦被
// load_tools 激活就带着完整 schema 进 tools 数组，模型仍然是照着 schema 填参数。

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"backend/internal/ai/core"
)

// loadToolsToolName 是那把"取 schema"的钥匙，它自己必须常驻工具集。
const loadToolsToolName = "load_tools"

var loadToolsToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: loadToolsToolName,
		Description: "按名字取回 MCP 工具的完整参数说明并激活它们。系统提示词里的「MCP 工具索引」" +
			"只给了名字和用途，要真正调用某个工具，先用这个把它加载进来（可一次传多个）。" +
			"加载后该工具就出现在你的可用工具列表里，直接照常调用即可。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"names": {
					Type:        "array",
					Description: "要加载的工具名数组，必须与索引里的名字完全一致，例如 [\"mcp__fs__edit_file\"]",
					Items:       &core.ToolProperty{Type: "string"},
				},
			},
			Required: []string{"names"},
		},
	},
}

// nativeWorkflowToolDefs 常驻工具：编排类的 dispatch_agent + load_tools/read_skill 这类
// 按需取全文的钥匙 + update_todo。文件读写/命令/检索/记忆全部走 MCP（按需 load_tools 加载），
// 内置工具已整体退役——主 Agent 靠 load_tools 拉 MCP 工具，子代理直接用 MCP 只读子集（见 subagent.go）。
// 这几个常驻是因为数量少、几乎每个任务都要用，藏进按需加载得不偿失。
func nativeWorkflowToolDefs() []core.ToolDefinition {
	return []core.ToolDefinition{dispatchAgentToolDef, loadToolsToolDef, updateTodoToolDef, readSkillToolDef}
}

// firstSentence 取描述的第一句，索引行只要一句话说清用途。
// 中英文标点都断，都没有就按长度硬截。
func firstSentence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	for _, sep := range []string{"。", ". ", "；", "; "} {
		if i := strings.Index(s, sep); i > 0 {
			return strings.TrimSpace(s[:i])
		}
	}
	return truncateChars(s, 110)
}

// mcpToolIndexPrompt 生成注入系统提示词的 MCP 工具索引（名字 + 一句话）。
// 这段本身是稳定内容（只在 MCP 增删时变），适合待在前缀里。
func mcpToolIndexPrompt() string {
	defs := loadMCPToolDefs()
	if len(defs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(defs))
	for _, t := range defs {
		lines = append(lines, fmt.Sprintf("- %s：%s", t.Function.Name, firstSentence(t.Function.Description)))
	}
	sort.Strings(lines)
	return "\n━━━ MCP 工具索引（按需加载） ━━━\n" +
		"下列工具的完整参数说明没有直接给你——需要用哪个，先调 load_tools 加载，加载后再正常调用。\n" +
		"一次可以加载多个；已加载的工具在后续轮次一直可用，不用重复加载。\n" +
		strings.Join(lines, "\n") + "\n"
}

// buildCodeWorkflowTools 组装本轮要发给模型的 tools 数组：
// 常驻工具 + 已被 load_tools 激活的 MCP 工具。activated 为 nil 时就只有常驻工具。
func buildCodeWorkflowTools(activated map[string]bool) []map[string]any {
	defs := nativeWorkflowToolDefs()
	if len(activated) > 0 {
		for _, t := range loadMCPToolDefs() {
			if activated[t.Function.Name] {
				defs = append(defs, t)
			}
		}
	}
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

// handleLoadTools 处理一次 load_tools 调用：把请求的工具标记为已激活，
// 并把它们的完整 schema 作为工具结果回给模型。
// 返回 (结果文本, 是否有工具真的被新激活)。
//
// 不存在的名字不是致命错误——回一句"没有这个工具"，让模型对着索引改，
// 比直接报错中断任务好；模型拼错名字是常见情况。
func handleLoadTools(argsJSON string, activated map[string]bool) (string, bool) {
	var args struct {
		Names []string `json:"names"`
		// 容错：模型有时会传单个字符串而不是数组
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "参数解析失败，names 应为字符串数组，例如 {\"names\":[\"mcp__fs__read_text_file\"]}", false
	}
	names := args.Names
	if len(names) == 0 && args.Name != "" {
		names = []string{args.Name}
	}
	if len(names) == 0 {
		return "names 为空，请指定要加载的工具名（见系统提示词里的 MCP 工具索引）", false
	}

	byName := map[string]core.ToolDefinition{}
	for _, t := range loadMCPToolDefs() {
		byName[t.Function.Name] = t
	}

	var loaded []map[string]any
	var missing []string
	changed := false
	for _, n := range names {
		t, ok := byName[n]
		if !ok {
			missing = append(missing, n)
			continue
		}
		if !activated[n] {
			activated[n] = true
			changed = true
		}
		loaded = append(loaded, map[string]any{
			"name":        t.Function.Name,
			"description": t.Function.Description,
			"parameters":  t.Function.Parameters,
		})
	}

	var b strings.Builder
	if len(loaded) > 0 {
		schemas, _ := json.MarshalIndent(loaded, "", "  ")
		fmt.Fprintf(&b, "已加载 %d 个工具，现在可以直接调用：\n%s", len(loaded), schemas)
	}
	if len(missing) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "以下名字在 MCP 工具索引里不存在：%s\n请对照系统提示词里的索引核对名字。",
			strings.Join(missing, "、"))
	}
	return b.String(), changed
}
