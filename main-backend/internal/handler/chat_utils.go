// handler/chat_utils.go - 增强版解析器 + 工具执行

package handler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

// 截断历史消息
func truncateHistory(history []DSMessage, maxHistory int) []DSMessage {
	if len(history) > maxHistory {
		return history[len(history)-maxHistory:]
	}
	return history
}

// 构建消息列表
func buildChatMessages(systemPrompt string, history []DSMessage, userMessage string) []map[string]string {
	msgs := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, msg := range history {
		msgs = append(msgs, map[string]string{"role": msg.Role, "content": msg.Content})
	}
	msgs = append(msgs, map[string]string{"role": "user", "content": userMessage})
	return msgs
}

// ToolCallRequest 本地模型工具调用结构
type ToolCallRequest struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

// 从文本中解析工具调用（支持严格 JSON 与混合文本内嵌 JSON）
func parseToolCallFromText(content string) (*ToolCallRequest, string, bool) {
	trimmed := strings.TrimSpace(content)

	// 处理 markdown 代码块包裹的情况：```json ... ``` 或 ``` ... ```
	if strings.HasPrefix(trimmed, "```json") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	} else if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	// 策略一：整段文本就是纯 JSON（严格模式）
	if strings.HasPrefix(trimmed, "{") {
		var tc ToolCallRequest
		if err := json.Unmarshal([]byte(trimmed), &tc); err == nil {
			if tc.Tool != "" && tc.Args != nil {
				return &tc, trimmed, true
			}
		}
	}

	// 策略二：从混合文本中提取内嵌的 {"tool":"...","args":{...}} JSON
	if idx := strings.Index(trimmed, `{"tool":"`); idx >= 0 {
		candidate := trimmed[idx:]
		depth := 0
		end := -1
		for i, ch := range candidate {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
		if end > 0 {
			jsonStr := candidate[:end]
			var tc ToolCallRequest
			if err := json.Unmarshal([]byte(jsonStr), &tc); err == nil {
				if tc.Tool != "" && tc.Args != nil {
					return &tc, jsonStr, true
				}
			}
		}
	}

	return nil, "", false
}
func parseJSONToolCallStrict(content string) (*ToolCallRequest, bool) {
	trimmed := strings.TrimSpace(content)

	// 必须是纯 JSON：以 { 开头，以 } 结尾
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return nil, false
	}

	// 不允许出现自然语言、emoji、markdown、代码块
	if strings.ContainsAny(trimmed, "，。！？：；`*#[]()<>\n\r") {
		return nil, false
	}

	var tc ToolCallRequest
	if err := json.Unmarshal([]byte(trimmed), &tc); err != nil {
		return nil, false
	}

	// 必须包含 tool 和 args
	if tc.Tool == "" || tc.Args == nil {
		return nil, false
	}

	return &tc, true
}
func parseXMLToolCallStrict(content string) (*ToolCallRequest, bool) {
	trimmed := strings.TrimSpace(content)

	// 必须严格以 <tool_call> 开头，以 </tool_call> 结尾
	if !strings.HasPrefix(trimmed, "<tool_call>") ||
		!strings.HasSuffix(trimmed, "</tool_call>") {
		return nil, false
	}

	// 提取中间 JSON
	inner := strings.TrimPrefix(trimmed, "<tool_call>")
	inner = strings.TrimSuffix(inner, "</tool_call>")
	inner = strings.TrimSpace(inner)

	// 中间必须是纯 JSON
	if !strings.HasPrefix(inner, "{") || !strings.HasSuffix(inner, "}") {
		return nil, false
	}

	var call struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(inner), &call); err != nil {
		return nil, false
	}

	if call.Name == "" {
		return nil, false
	}

	return &ToolCallRequest{
		Tool: call.Name,
		Args: call.Arguments,
	}, true
}

// 解析 JSON 格式：{"tool":"xxx","args":{...}}
func parseJSONToolCall(content string) (*ToolCallRequest, bool) {
	if !strings.Contains(content, `"tool"`) {
		return nil, false
	}
	start := strings.Index(content, `{`)
	end := strings.LastIndex(content, `}`) + 1
	if start < 0 || end <= start {
		return nil, false
	}
	jsonStr := content[start:end]
	var tc ToolCallRequest
	if err := json.Unmarshal([]byte(jsonStr), &tc); err != nil {
		return nil, false
	}
	return &tc, true
}

// 解析 XML 标签格式：<tool_call>{"name":"read_file","arguments":{"path":"..."}}</tool_call>
func parseXMLToolCall(content string) (*ToolCallRequest, bool) {
	startTag := "<tool_call>"
	endTag := "</tool_call>"
	start := strings.Index(content, startTag)
	end := strings.Index(content, endTag)
	if start < 0 || end < 0 || end <= start {
		return nil, false
	}
	jsonStr := content[start+len(startTag) : end]
	// XML 内的 JSON 格式可能是 {"name":"xxx","arguments":{...}}
	var call struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &call); err != nil {
		return nil, false
	}
	args := call.Arguments
	if args == nil {
		args = make(map[string]interface{})
	}
	// 兼容 command 和 cmd 参数
	if cmd, ok := args["command"].(string); ok {
		args["command"] = cmd
	} else if cmd, ok := args["cmd"].(string); ok {
		args["command"] = cmd
	}
	return &ToolCallRequest{
		Tool: call.Name,
		Args: args,
	}, true
}

// 执行工具并通知前端
func (h *ChatHandler) executeToolAndNotify(c *gin.Context, sessionID string, finalContent string, toolCall ToolCallRequest) (string, error) {
	// 兼容模型可能输出的 cmd 参数名，统一转为 command
	args := toolCall.Args
	if _, hasCmd := args["cmd"]; hasCmd {
		if _, hasCommand := args["command"]; !hasCommand {
			args["command"] = args["cmd"]
		}
	}

	argsBytes, _ := json.Marshal(args)
	call := core.ToolCall{
		ID:   "agent_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Type: "function",
		Function: core.ToolCallFunc{
			Name:      toolCall.Tool,
			Arguments: string(argsBytes),
		},
	}

	result, err := core.ExecuteToolCall(call)
	if err != nil || result == nil {
		writeSSE(c, "tool_call", "tool_call_error", map[string]string{
			"name":  toolCall.Tool,
			"error": fmt.Sprintf("%v", err),
		})
		c.Writer.Flush()
		return "", err
	}

	eventType := "tool_call_result"
	if result.Failed {
		eventType = "tool_call_error"
	}
	writeSSE(c, "tool_call", eventType, map[string]string{
		"name":   toolCall.Tool,
		"result": result.Content,
	})
	c.Writer.Flush()

	h.sessionStore.Append(sessionID, DSMessage{Role: "assistant", Content: finalContent, Model: "local"})
	h.sessionStore.Append(sessionID, DSMessage{Role: "tool", Content: result.Content, Model: "local"})

	return result.Content, nil
}

// 静默执行工具，不推送任何 SSE
func executeToolSilently(sessionID string, toolCall ToolCallRequest) (string, error) {
	args := toolCall.Args
	if _, hasCmd := args["cmd"]; hasCmd {
		if _, hasCommand := args["command"]; !hasCommand {
			args["command"] = args["cmd"]
		}
	}
	argsBytes, _ := json.Marshal(args)
	call := core.ToolCall{
		ID:   "agent_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Type: "function",
		Function: core.ToolCallFunc{
			Name:      toolCall.Tool,
			Arguments: string(argsBytes),
		},
	}
	result, err := core.ExecuteToolCall(call)
	if err != nil || result == nil {
		return "", err
	}
	return result.Content, nil
}

// 将任意对象序列化为 JSON 字符串
func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// formatToolArgs 将 map[string]interface{} 格式化为 key="value" 键值对字符串
// 例：{"path": "a.py", "content": "print(1)"} → path="a.py" content="print(1)"
// 前端用 /(\w+)="([\s\S]*?)"/g 解析，value 内部的双引号会被转义
func formatToolArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys) // 确定性顺序，便于前端调试
	var parts []string
	for _, k := range keys {
		v := args[k]
		val := fmt.Sprintf("%v", v)
		// 转义 value 内部的双引号：\" → \\" 这样 JSON 序列化后前端
		// 反序列化得到的是 \"，正则中的 ? 会停在反斜杠这里不被吞掉
		val = strings.ReplaceAll(val, `"`, `\"`)
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, val))
	}
	return strings.Join(parts, " ")
}

// stripToolJSON 从内容中移除找到的工具调用 JSON，返回纯叙述文本
func stripToolJSON(content string, jsonStr string) string {
	if jsonStr == "" {
		return content
	}
	idx := strings.Index(content, jsonStr)
	if idx < 0 {
		return content
	}
	before := strings.TrimSpace(content[:idx])
	after := strings.TrimSpace(content[idx+len(jsonStr):])
	result := before
	if after != "" {
		if result != "" {
			result += "\n"
		}
		result += after
	}
	return result
}
