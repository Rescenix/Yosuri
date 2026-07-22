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
	// Items 描述 type:"array" 时的元素类型。缺了它，多数上游会判定 array 参数
	// schema 不合法（或让模型自由发挥填出五花八门的元素），所以数组参数必须带。
	Items *ToolProperty `json:"items,omitempty"`
}

// 内置原生工具已整体退役：文件/命令/检索/记忆全部走 MCP（见 handler/tool_ondemand.go、
// subagent.go）。本文件只保留工具定义的数据结构（上面的 ToolDefinition 等）与
// DS 浏览器标记解析辅助（下方 unescapeToolMarker/ExtractToolArgs）。

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
