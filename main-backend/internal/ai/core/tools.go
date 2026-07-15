package core

import (
	"fmt"
	"strings"
)

// ToolDefinition 是一个通用的工具定义结构，最终会序列化为 JSON 传给 API
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function ToolFunctionDetail `json:"function"`
}

type ToolFunctionDetail struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolParameters struct {
	Type       string                  `json:"type"`
	Properties map[string]ToolProperty `json:"properties"`
	Required   []string                `json:"required"`
}

type ToolProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// BaseTools 是电脑和手机共用的工具
var BaseTools = []ToolDefinition{
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "search_codebase",
			Description: "语义搜索代码库，返回与查询最相关的函数、模块和代码片段。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "搜索查询，例如 '用户登录逻辑' 或 '向量检索实现'",
					},
				},
				Required: []string{"query"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "read_file",
			Description: "读取项目中的指定文件。默认返回完整文本；可选 start_line/end_line 只读某段行（1-indexed 闭区间），或 mode=\"outline\" 只看函数/类/导出的签名骨架（不含函数体，每行带行号）。建议先用 outline 看结构、再用行范围拉正文，避免整文件塞入上下文。行范围与 outline 互斥。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "文件路径，相对于项目根目录（如 'internal/ai/core/tools.go'）",
					},
					"start_line": {
						Type:        "integer",
						Description: "可选，起始行号（1-indexed，闭区间）。越界自动裁剪。不能与 mode=outline 同用。",
					},
					"end_line": {
						Type:        "integer",
						Description: "可选，结束行号（1-indexed，闭区间）。越界自动裁剪到文件末行。不能与 mode=outline 同用。",
					},
					"mode": {
						Type:        "string",
						Description: "可选，读取模式：full（默认，返回正文）或 outline（只返回签名骨架，含行号）。outline 不能与 start_line/end_line 同用。",
						Enum:        []string{"full", "outline"},
					},
				},
				Required: []string{"path"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "write_file",
			Description: "创建或覆盖项目中的文件。此操作需要用户确认。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "文件路径，相对于项目根目录",
					},
					"content": {
						Type:        "string",
						Description: "要写入的文件内容",
					},
				},
				Required: []string{"path", "content"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "edit_file",
			Description: "精确编辑文件内容，通过替换指定旧字符串为新字符串实现修改。保留 diff 信息，要求 old_string 在文件中唯一。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "要编辑的文件路径，相对于项目根目录",
					},
					"old_string": {
						Type:        "string",
						Description: "要替换的旧字符串，必须在文件中唯一匹配",
					},
					"new_string": {
						Type:        "string",
						Description: "替换后的新字符串",
					},
				},
				Required: []string{"path", "old_string", "new_string"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "list_dir",
			Description: "列出目录下的文件和子目录（只读）。比 execute_command 跑 dir/ls 更省 token：输出紧凑、自动跳过 node_modules/.git 等噪声目录。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "目录路径，相对项目根目录或绝对路径（如 'main-backend/internal'）",
					},
					"recursive": {
						Type:        "boolean",
						Description: "可选，true 递归列出子目录内容（默认 false 只列一层）",
					},
				},
				Required: []string{"path"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "execute_command",
			Description: "在项目根目录执行一条安全的白名单 shell 命令。需要用户确认。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"command": {
						Type:        "string",
						Description: "要执行的 shell 命令（如 'git diff'、'go build ./...'）",
					},
				},
				Required: []string{"command"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "search_memory",
			Description: "检索历史内容，分两种作用域(scope)：scope=\"sessions\" 在全部历史会话消息里正则/子串检索（默认，最常用，能找回任何聊过的事）；scope=\"memory\" 检索长期记忆卡片(MEMORY.md)。sessions 模式下 query 作为正则表达式匹配（非法正则自动退化为子串匹配），返回命中片段含 session_id/时间/角色/上下文；可加 mode=\"detail\" 配合 session_id 拉该会话更多上下文，或传 id 在 memory 模式下展开记忆卡片。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "检索式：sessions 模式下为正则表达式（如 'mcp|MCP'、'读取流失败'），memory 模式下为记忆检索词。",
					},
					"scope": {
						Type:        "string",
						Description: "检索作用域：sessions=全部历史会话消息（默认、最常用）；memory=长期记忆卡片(MEMORY.md)。",
						Enum:        []string{"sessions", "memory"},
					},
					"mode": {
						Type:        "string",
						Description: "memory 模式专用：summary 返回摘要列表（默认），detail 按 id 返回完整内容。sessions 模式忽略。",
						Enum:        []string{"summary", "detail"},
					},
					"id": {
						Type:        "string",
						Description: "memory 模式 detail 下要展开的记忆 ID（0x 开头的十六进制串）。",
					},
					"session_id": {
						Type:        "string",
						Description: "可选：限定只在某个会话里检索（sessions 模式）。不传则搜全部会话。",
					},
					"limit": {
						Type:        "integer",
						Description: "可选：返回的最大命中条数（默认 20）。",
					},
				},
				Required: []string{},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "clean_memories",
			Description: "清理冗余或过时的记忆，优化记忆库,禁止主动调用。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
		},
	},
}

// WindowsTools 是 Windows 电脑端专有的工具
var WindowsTools = []ToolDefinition{
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "codegraph_query",
			Description: "使用 CodeGraph 查询代码库的结构信息（如 callers, callees, impact, context 等）",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"subcommand": {
						Type:        "string",
						Description: "CodeGraph 子命令（callers, callees, impact, context, search, status 等）",
					},
					"symbol": {
						Type:        "string",
						Description: "要查询的符号名称（如函数名、类名）",
					},
				},
				Required: []string{"subcommand", "symbol"},
			},
		},
	},
}

// ChatTools 默认是 Windows 全量工具
var ChatTools []ToolDefinition

func init() {
	ChatTools = append(ChatTools, BaseTools...)
	ChatTools = append(ChatTools, WindowsTools...)
}

// unescapeToolMarker 还原 DS 浏览器返回的转义字符
func unescapeToolMarker(raw string) string {
	raw = strings.ReplaceAll(raw, "\\[TOOL:", "[TOOL:")
	raw = strings.ReplaceAll(raw, "\\]", "]")
	raw = strings.ReplaceAll(raw, "\\_", "_")
	raw = strings.ReplaceAll(raw, "\\\"", "\"")
	raw = strings.ReplaceAll(raw, "\\'", "'")
	raw = strings.ReplaceAll(raw, "\\\\", "\\") // 双反斜杠→单反斜杠，务必放最后
	return raw
}

func ExtractToolArgs(marker string) (string, map[string]string, error) {
	// 不再做任何 unescape，前端已经给的是干净的标记
	if !strings.HasPrefix(marker, "[TOOL:") || !strings.HasSuffix(marker, "]") {
		return "", nil, fmt.Errorf("not a tool marker")
	}

	inner := marker[6 : len(marker)-1] // 去掉 [TOOL: 和 ]
	parts := strings.SplitN(inner, " ", 2)
	toolName := parts[0]
	args := make(map[string]string)

	if len(parts) < 2 {
		return toolName, args, nil
	}

	remainder := parts[1] // 例如：path="C:\Pro2026\re0\.gitignore"

	// 遍历所有 key="value" 对
	i := 0
	for i < len(remainder) {
		// 跳过空格
		for i < len(remainder) && remainder[i] == ' ' {
			i++
		}
		if i >= len(remainder) {
			break
		}

		// 找到等号
		eqIdx := strings.Index(remainder[i:], "=")
		if eqIdx == -1 {
			break
		}
		eqIdx += i
		key := strings.TrimSpace(remainder[i:eqIdx])
		i = eqIdx + 1 // 移到等号后

		// 值必须由引号包围
		if i >= len(remainder) || remainder[i] != '"' {
			return "", nil, fmt.Errorf("value must start with quote")
		}
		i++ // 跳过开始的引号

		// 找到闭合引号：下一个未被反斜杠转义的引号。
		// 之前用 LastIndex 找"最后一个引号"，单参数时碰巧工作，
		// 多参数 marker（如 mode="detail" id="0x1a"）会把后续参数整段吞进第一个值里
		endQuote := -1
		for j := i; j < len(remainder); j++ {
			if remainder[j] == '"' && (j == i || remainder[j-1] != '\\') {
				endQuote = j - i
				break
			}
		}
		if endQuote == -1 {
			return "", nil, fmt.Errorf("unclosed quote")
		}
		value := remainder[i : i+endQuote]
		// 只将内部的 \" 还原为 "，其他保持不变
		value = strings.ReplaceAll(value, `\"`, `"`)
		args[key] = value
		i = i + endQuote + 1 // 移到闭合引号之后
	}

	return toolName, args, nil
}
