package main

// tools.go — 工具调用框架（从 re0 main-backend/internal/ai/core 移植）
//
// Rescene 自研 [TOOL:name key="value"] marker 协议：
//   模型在回复里写标记 → 后端解析 → 执行工具 → 结果喂回模型继续推理
// 不依赖 OpenAI function calling，任何模型都能用。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolDefinition 工具定义（序列化为 JSON 传给模型）
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
	// omitempty 必须：无必填参数时这里是 nil，不加 omitempty 会序列化成
	// "required": null，DeepSeek 等上游 schema 校验直接拒整个请求
	Required []string `json:"required,omitempty"`
}

type ToolProperty struct {
	Type        string                  `json:"type"`
	Description string                  `json:"description,omitempty"`
	Enum        []string                `json:"enum,omitempty"`
	Properties  map[string]ToolProperty `json:"properties,omitempty"`
	Required    []string                `json:"required,omitempty"`
	MinItems    int                     `json:"minItems,omitempty"`
	Items       *ToolProperty           `json:"items,omitempty"`
}

// ToolResult 工具执行结果
type ToolResult struct {
	Text   string
	Images []string // base64 PNG（computer_screenshot 等）
	URLs   []string // 来源引用（web_search）
}

// nativeTool 构建工具定义
func nativeTool(name, description string, properties map[string]ToolProperty, required []string) ToolDefinition {
	return ToolDefinition{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        name,
			Description: description,
			Parameters: ToolParameters{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
		},
	}
}

// allToolDefs 返回全部可用工具定义
func allToolDefs() []ToolDefinition {
	return append(nativeToolDefs(), loadMCPToolDefs()...)
}

// toolDefsJSON 序列化工具定义给模型
func toolDefsJSON() string {
	defs := allToolDefs()
	if len(defs) == 0 {
		return ""
	}
	b, err := json.Marshal(defs)
	if err != nil {
		return ""
	}
	return string(b)
}

// ExtractToolArgs 解析 [TOOL:name key="value" key2="value2"] 标记
// 返回工具名和参数 map
func ExtractToolArgs(marker string) (string, map[string]string, error) {
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

	remainder := parts[1]
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
		i = eqIdx + 1

		// 值必须由引号包围
		if i >= len(remainder) || remainder[i] != '"' {
			return "", nil, fmt.Errorf("value must start with quote")
		}
		i++
		// 找到闭合引号：下一个未被反斜杠转义的引号
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
		value = strings.ReplaceAll(value, `\"`, `"`)
		args[key] = value
		i = i + endQuote + 1
	}
	return toolName, args, nil
}

// ExtractToolMarkers 从回复文本中提取所有 [TOOL:...] 标记
func ExtractToolMarkers(content string) []string {
	var out []string
	rest := content
	for {
		start := strings.Index(rest, "[TOOL:")
		if start == -1 {
			break
		}
		end := strings.Index(rest[start:], "]")
		if end == -1 {
			break
		}
		marker := rest[start : start+end+1]
		out = append(out, marker)
		rest = rest[start+end+1:]
	}
	return out
}

// callTool 执行工具，返回结果
func callTool(ctx context.Context, name string, args map[string]string) (ToolResult, error) {
	// nil ctx 兜底（skill 记录过的坑：context.WithTimeout(nil) 会 panic）
	if ctx == nil {
		ctx = context.Background()
	}
	argsJSON, _ := json.Marshal(args)
	return callNativeTool(ctx, name, string(argsJSON))
}
