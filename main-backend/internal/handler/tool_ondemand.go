package handler

// 工具按需加载 —— 默认只把 Go 内置工具和外部 MCP 工具的「名字 + 一句话」放进上下文，
// 模型要用哪个再调 load_tools 取回完整 schema，之后该工具才进 tools 数组。
//
// 为什么：实测（context_budget_test.go）全量 schema 占 6563 tok，是整个静态前缀的 91%，
// 而且每轮都要重发一遍——20 轮就是 13 万 token 只为反复描述工具长什么样。
// 一个任务真正会用到的通常是 2-4 个工具，剩下 25 个的 schema 是纯粹的浪费。
//
// 为什么不用「万能代理工具」（call_tool(name, args)）：那样模型是照着一句话描述
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
		Description: "按名字取回 Go 内置或外部 MCP 工具的完整参数说明并激活它们。系统提示词里的「按需工具索引」" +
			"只给了名字和用途，要真正调用某个工具，先用这个把它加载进来（可一次传多个）。" +
			"加载后该工具就出现在你的可用工具列表里，直接照常调用即可。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"names": {
					Type:        "array",
					Description: "要加载的工具名数组，必须与索引里的名字完全一致，例如 [\"read_file\",\"edit_file\"]",
					Items:       &core.ToolProperty{Type: "string"},
				},
			},
			Required: []string{"names"},
		},
	},
}

// nativeWorkflowToolDefs 常驻工具：编排类的 dispatch_agent + load_tools/read_skill 这类
// 按需取全文的钥匙 + update_todo。文件读写/命令/检索/记忆由 Go 内置工具提供，
// 同样通过 load_tools 按需激活；真正的外部能力仍可由 MCP 扩展。
// 这几个常驻是因为数量少、几乎每个任务都要用，藏进按需加载得不偿失。
func nativeWorkflowToolDefs() []core.ToolDefinition {
	return []core.ToolDefinition{
		// apply_patch 是长文件写入的基础能力，必须从第一轮起直接可用。
		// 模型可以先建骨架，再用多个小补丁逐段追加，避开单次 write_file 参数过长。
		nativeApplyPatchToolDef,
		dispatchAgentToolDef, loadToolsToolDef, updateTodoToolDef, readSkillToolDef, skillManageToolDef,
		// harness_status：让模型能问"我上下文里现在有什么、丢了什么、去哪捞"。
		// 常驻是有意的——正因为它是自省用的，模型需要它的时候恰恰是"感觉不对劲"
		// 的时候，那时再让它先 load_tools 就晚了。
		harnessStatusToolDef,
		// ask_user：让 agent 在工作流中途向用户提问并暂停等待回答（human-in-the-loop）。
		// 常驻是因为它是交互控制面工具，需要用时再 load_tools 就打断节奏了。
		askUserToolDef,
		// capture_preview：截「用户正在看的内嵌预览页」发聊天，harness 内置、常驻。
		capturePreviewToolDef,
		// open_preview：agent 主动把指定页面弹进内嵌预览面板（harness CDP 通道），
		// 区别于系统收尾自动弹——主动权在 agent。常驻，无需 load_tools。
		openPreviewToolDef,
		// 通用页面注入：供前端设计 Agent 在当前预览页执行交互或验证脚本。
		injectPreviewToolDef,
	}
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

// mcpToolIndexPrompt 保留旧函数名以减少调用面变更；它现在生成全部按需工具索引。
func mcpToolIndexPrompt() string {
	defs := allOnDemandToolDefs()
	if len(defs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(defs))
	for _, t := range defs {
		lines = append(lines, fmt.Sprintf("- %s：%s", t.Function.Name, firstSentence(t.Function.Description)))
	}
	sort.Strings(lines)
	return "\n━━━ 按需工具索引（Go 内置 + 外部 MCP） ━━━\n" +
		"下列工具的完整参数说明没有直接给你——需要用哪个，先调 load_tools 加载，加载后再正常调用。\n" +
		"一次可以加载多个；已加载的工具在后续轮次一直可用，不用重复加载。\n" +
		"截图/页面自检/浏览器快照成功时会直接作为图片插入当前聊天消息流；不要声称无法贴图、不要要求用户另存文件。\n" +
		strings.Join(lines, "\n") + "\n"
}

// nativeToolIndexPrompt 生成常驻原生工具（不按需加载、一直进 tools 数组）的索引。
// 为什么需要它：mcpToolIndexPrompt 只列按需工具，常驻原生工具（dispatch_agent /
// load_tools 等）的 schema 走 tools 参数但**不进系统提示词索引**，模型在索引里
// 看不到它们，就会以为"我没有这个工具"而绕路。这里把它们也列成「名字 + 一句话」，
// 让模型知道存在、知道何时直接调。
func nativeToolIndexPrompt() string {
	defs := nativeWorkflowToolDefs()
	if len(defs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(defs))
	for _, t := range defs {
		lines = append(lines, fmt.Sprintf("- %s：%s", t.Function.Name, firstSentence(t.Function.Description)))
	}
	sort.Strings(lines)
	return "\n━━━ 常驻原生工具（始终可用，直接调用，无需 load_tools） ━━━\n" +
		strings.Join(lines, "\n") + "\n"
}

// buildCodeWorkflowTools 组装本轮要发给模型的 tools 数组：
// 常驻工具 + 已被 load_tools 激活的 Go 内置/MCP 工具。activated 为 nil 时就只有常驻工具。
func buildCodeWorkflowTools(activated map[string]bool) []map[string]any {
	defs := nativeWorkflowToolDefs()
	if len(activated) > 0 {
		for _, t := range allOnDemandToolDefs() {
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
		return "参数解析失败，names 应为字符串数组，例如 {\"names\":[\"read_file\",\"grep\"]}", false
	}
	names := args.Names
	if len(names) == 0 && args.Name != "" {
		names = []string{args.Name}
	}
	if len(names) == 0 {
		return "names 为空，请指定要加载的工具名（见系统提示词里的按需工具索引）", false
	}

	byName := map[string]core.ToolDefinition{}
	for _, t := range allOnDemandToolDefs() {
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

	// 常驻工具被拿来 load 是很常见的一类误解（"是不是所有工具都得先加载"）。
	// 系统明明知道答案——它已经在工具数组里了——就不该回一句"名字不存在，
	// 自己去核对索引"把模型支走。直接告诉它可以马上调。
	var resident []string
	stillMissing := missing[:0]
	for _, n := range missing {
		if isNativeWorkflowTool(n) {
			resident = append(resident, n)
		} else {
			stillMissing = append(stillMissing, n)
		}
	}

	var b strings.Builder
	if len(loaded) > 0 {
		schemas, _ := json.MarshalIndent(loaded, "", "  ")
		fmt.Fprintf(&b, "已加载 %d 个工具，现在可以直接调用：\n%s", len(loaded), schemas)
	}
	if len(resident) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "%s 是常驻工具，不需要加载——它一直就在你的工具列表里，直接调用即可。",
			strings.Join(resident, "、"))
	}
	if len(stillMissing) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "以下名字在按需工具索引里不存在：%s\n请对照系统提示词里的索引核对名字。",
			strings.Join(stillMissing, "、"))
	}
	return b.String(), changed
}

// isNativeWorkflowTool 判断一个名字是不是常驻工具（无需 load_tools）。
func isNativeWorkflowTool(name string) bool {
	for _, t := range nativeWorkflowToolDefs() {
		if t.Function.Name == name {
			return true
		}
	}
	return false
}
