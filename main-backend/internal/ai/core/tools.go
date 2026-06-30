package core

import (
	"fmt"
	"regexp"
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
			Description: "读取项目中的指定文件内容，返回完整文本。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "文件路径，相对于项目根目录（如 'internal/ai/core/tools.go'）",
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

// extractToolArgs 从 "[TOOL:xxx key="val" ...]" 中提取工具名和参数
func ExtractToolArgs(marker string) (string, map[string]string, error) {
	// 还原 DS 转义
	marker = unescapeToolMarker(marker)

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

	// 对于 execute_command，单独提取 command 参数
	if toolName == "execute_command" {
		// 找到 command=" 的位置
		cmdStart := strings.Index(parts[1], `command="`)
		if cmdStart == -1 {
			return toolName, nil, fmt.Errorf("missing command parameter")
		}
		// 从 command=" 之后开始截取
		cmdRemainder := parts[1][cmdStart+len(`command="`):]
		// 命令的结束是标记末尾之前的一个双引号，这里已经是 inner 里，所以结束引号在末尾之前
		// 因为标记格式是 [TOOL:execute_command command="..."]，内层 inner 是 execute_command command="..."]
		// 所以直接截取到倒数第二个字符（忽略尾随的 "]）
		// 正确做法：找到最后一个双引号（即命令的闭合引号），但不能包括末尾可能多出的引号。
		// 由于我们已经做了 unescape，标记结尾应该是 ...command="dir "C:\...""] 这种形式。
		// 简单方案：从后往前找第一个双引号，它就是命令参数的结束引号。
		lastQuote := strings.LastIndex(cmdRemainder, `"`)
		if lastQuote > 0 {
			args["command"] = cmdRemainder[:lastQuote]
		} else {
			args["command"] = cmdRemainder
		}
		return toolName, args, nil
	}

	// 其他工具用简单正则
	re := regexp.MustCompile(`(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(parts[1], -1)
	for _, m := range matches {
		if len(m) == 3 {
			args[m[1]] = m[2]
		}
	}
	return toolName, args, nil
}
