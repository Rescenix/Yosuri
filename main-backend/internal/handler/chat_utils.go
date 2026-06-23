// handler/chat_utils.go - 增强版解析器 + 工具执行

package handler

import (
	"encoding/json"
	"fmt"
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

// 从文本中解析工具调用（支持 JSON + XML 两种格式）
func parseToolCallFromText(content string) (*ToolCallRequest, bool) {
	// 先尝试 JSON 格式
	if tc, ok := parseJSONToolCall(content); ok {
		return tc, true
	}
	// 再尝试 XML 标签格式
	if tc, ok := parseXMLToolCall(content); ok {
		return tc, true
	}
	return nil, false
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

	h.sessionStore.Append(sessionID, DSMessage{Role: "assistant", Content: finalContent})
	h.sessionStore.Append(sessionID, DSMessage{Role: "tool", Content: result.Content})

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
