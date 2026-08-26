package handler

// aggregate_responses_convert.go —— Responses 协议 ↔ chat/completions 翻译函数。

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

// responsesInputToMessages 把 Responses 的 input（字符串或 items 数组）翻译成
// chat/completions messages。
//   - message item → 普通消息（content 数组里 input_text 拼文本、input_image 转 image_url）
//   - function_call item → 并入前一条 assistant 消息的 tool_calls
//   - function_call_output item → tool 消息
//   - instructions → 最前插一条 system 消息
func responsesInputToMessages(raw json.RawMessage, instructions string) ([]map[string]any, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		msgs := []map[string]any{{"role": "user", "content": s}}
		if instructions != "" {
			msgs = append([]map[string]any{{"role": "system", "content": instructions}}, msgs...)
		}
		return msgs, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("input 必须是字符串或 items 数组")
	}
	var msgs []map[string]any
	for _, it := range items {
		typ, _ := it["type"].(string)
		switch typ {
		case "message":
			role, _ := it["role"].(string)
			if role == "" {
				role = "user"
			}
			msgs = append(msgs, map[string]any{"role": role, "content": responsesItemToChatContent(it["content"])})
		case "function_call":
			// Responses 协议里 function_call 应紧跟所属 assistant message，
			// 但部分客户端省略 message 直接发 call：无 assistant 可挂时自动补一条
			if len(msgs) == 0 {
				msgs = append(msgs, map[string]any{"role": "assistant", "content": ""})
			}
			last := msgs[len(msgs)-1]
			if r, _ := last["role"].(string); r != "assistant" {
				msgs = append(msgs, map[string]any{"role": "assistant", "content": ""})
				last = msgs[len(msgs)-1]
			}
			callID, _ := it["call_id"].(string)
			name, _ := it["name"].(string)
			args, _ := it["arguments"].(string)
			if callID == "" {
				callID = fmt.Sprintf("call_%d", len(msgs))
			}
			tc := map[string]any{
				"id": callID, "type": "function",
				"function": map[string]any{"name": name, "arguments": args},
			}
			var calls []map[string]any
			if rawCalls, ok := last["tool_calls"]; ok && rawCalls != nil {
				if bs, err := json.Marshal(rawCalls); err == nil {
					_ = json.Unmarshal(bs, &calls)
				}
			}
			calls = append(calls, tc)
			last["tool_calls"] = calls
		case "function_call_output":
			callID, _ := it["call_id"].(string)
			output, _ := it["output"].(string)
			if callID == "" {
				continue
			}
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": callID, "content": output})
		}
	}
	if instructions != "" {
		msgs = append([]map[string]any{{"role": "system", "content": instructions}}, msgs...)
	}
	return msgs, nil
}

// responsesItemToChatContent 把 Responses 的 content（字符串或 parts 数组）翻译成
// chat/completions 的 content（纯文本字符串，或含 text/image_url 的 parts 数组）。
func responsesItemToChatContent(content any) any {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	bs, err := json.Marshal(content)
	if err != nil {
		return ""
	}
	var parts []map[string]any
	if err := json.Unmarshal(bs, &parts); err != nil {
		return string(bs)
	}
	var textSb strings.Builder
	var out []map[string]any
	hasImage := false
	for _, p := range parts {
		pt, _ := p["type"].(string)
		switch pt {
		case "input_text", "output_text":
			textSb.WriteString(fmt.Sprint(p["text"]))
		case "input_image":
			hasImage = true
			url := fmt.Sprint(p["image_url"])
			if url == "<nil>" || url == "" {
				url = fmt.Sprint(p["url"])
			}
			out = append(out, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}})
		}
	}
	if !hasImage {
		return textSb.String()
	}
	if textSb.Len() > 0 {
		out = append([]map[string]any{{"type": "text", "text": textSb.String()}}, out...)
	}
	return out
}

// responsesToChatTools 把 Responses 工具格式（{type:function, name, description,
// parameters}）翻译成 chat/completions 格式（{type:function, function:{...}}）。
// 若调用方已发 chat 嵌套格式则原样保留。
func responsesToChatTools(tools []map[string]any) []map[string]any {
	var out []map[string]any
	for _, t := range tools {
		typ, _ := t["type"].(string)
		if typ != "function" {
			continue
		}
		if _, ok := t["function"].(map[string]any); ok {
			out = append(out, t)
			continue
		}
		fn := map[string]any{"name": t["name"], "description": t["description"]}
		if params, ok := t["parameters"].(map[string]any); ok && params != nil {
			fn["parameters"] = params
		}
		out = append(out, map[string]any{"type": "function", "function": fn})
	}
	return out
}

// buildAggregateResponses 把内部调用结果组装成 Responses 对象（非流式响应体）。
func buildAggregateResponses(b RouterBackend, content string, calls []core.ToolCall) gin.H {
	now := time.Now()
	id := fmt.Sprintf("resp_%d", now.UnixNano())
	var output []map[string]any
	if content != "" {
		output = append(output, map[string]any{
			"type": "message", "id": fmt.Sprintf("msg_%d", now.UnixNano()),
			"status": "completed", "role": "assistant",
			"content": []map[string]any{{"type": "output_text", "text": content, "annotations": []any{}}},
		})
	}
	for _, tc := range calls {
		output = append(output, map[string]any{
			"type": "function_call", "id": "fc_" + tc.ID, "call_id": tc.ID,
			"status": "completed", "name": tc.Function.Name, "arguments": tc.Function.Arguments,
		})
	}
	return gin.H{
		"id": id, "object": "response", "created_at": now.Unix(),
		"status": "completed", "model": b.Model, "output": output,
		"usage": gin.H{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0},
	}
}