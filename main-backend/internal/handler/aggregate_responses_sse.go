package handler

// aggregate_responses_sse.go —— 上游 chat/completions SSE → Responses SSE 事件翻译。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// responsesSSEState 累积流式翻译状态。
type responsesSSEState struct {
	c       *gin.Context
	flusher http.Flusher
	b       RouterBackend
	seq     int

	respID    string
	msgID     string
	createdAt int64

	text        strings.Builder
	textStarted bool
	textIdx     int // message 的 output_index（恒 0，message 排最前）

	fcSeen   map[int]bool
	fcArgs   map[int]string
	fcName   map[int]string
	fcCallID map[int]string

	output []map[string]any
}

func (s *responsesSSEState) emit(ev map[string]any) {
	ev["sequence_number"] = s.seq
	s.seq++
	payload, _ := json.Marshal(ev)
	fmt.Fprintf(s.c.Writer, "data: %s\n\n", payload)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *responsesSSEState) respMeta(status string) map[string]any {
	return map[string]any{
		"id": s.respID, "object": "response", "created_at": s.createdAt,
		"status": status, "model": s.b.Model, "output": s.output,
	}
}

// aggregateForwardResponsesSSE 把上游 chat/completions SSE 逐 chunk 翻译成
// Responses 协议 SSE 事件流（response.created → output_text.delta /
// function_call_arguments.delta → response.completed）。
func aggregateForwardResponsesSSE(c *gin.Context, b RouterBackend, resp *http.Response) {
	defer resp.Body.Close()
	circuitSuccess(b)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)

	s := &responsesSSEState{
		c: c, flusher: flusher, b: b,
		respID:    fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		msgID:     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		createdAt: time.Now().Unix(),
		fcSeen:    map[int]bool{},
		fcArgs:    map[int]string{},
		fcName:    map[int]string{},
		fcCallID:  map[int]string{},
	}

	s.emit(map[string]any{"type": "response.created", "response": s.respMeta("in_progress")})
	s.emit(map[string]any{"type": "response.in_progress", "response": s.respMeta("in_progress")})

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			s.finish()
			return
		}
		var chunk struct {
			Choices []struct {
				Delta        map[string]any `json:"delta"`
				FinishReason string         `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if len(delta) > 0 {
			s.handleDelta(delta)
		}
		if chunk.Choices[0].FinishReason != "" {
			s.finish()
			return
		}
	}
	// 流结束（无 [DONE] / 空响应）：照常收尾
	s.finish()
}

// handleDelta 处理单个上游 delta（content 文本 / tool_calls 工具调用）。
func (s *responsesSSEState) handleDelta(delta map[string]any) {
	if content, ok := delta["content"].(string); ok && content != "" {
		if !s.textStarted {
			s.textStarted = true
			s.emit(map[string]any{"type": "response.output_item.added", "output_index": s.textIdx, "item": map[string]any{
				"id": s.msgID, "type": "message", "status": "in_progress",
				"role": "assistant", "content": []any{},
			}})
			s.emit(map[string]any{"type": "response.content_part.added", "item_id": s.msgID,
				"output_index": s.textIdx, "content_index": 0,
				"part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
		}
		s.text.WriteString(content)
		s.emit(map[string]any{"type": "response.output_text.delta", "item_id": s.msgID,
			"output_index": s.textIdx, "content_index": 0, "delta": content})
		return
	}
	rawCalls, ok := delta["tool_calls"]
	if !ok || rawCalls == nil {
		return
	}
	bs, err := json.Marshal(rawCalls)
	if err != nil {
		return
	}
	var calls []map[string]any
	if err := json.Unmarshal(bs, &calls); err != nil {
		return
	}
	for _, call := range calls {
		idx := intFromAny(call["index"])
		if !s.fcSeen[idx] {
			s.fcSeen[idx] = true
			id := fmt.Sprintf("fc_%d", idx)
			callID, _ := call["id"].(string)
			if callID == "" {
				callID = id
			}
			s.fcCallID[idx] = callID
			name, _ := mapString(call, "function", "name")
			s.fcName[idx] = name
			s.emit(map[string]any{"type": "response.output_item.added", "output_index": s.textIdx + 1 + idx, "item": map[string]any{
				"id": id, "type": "function_call", "status": "in_progress",
				"call_id": callID, "name": name, "arguments": "",
			}})
		}
		fn, _ := call["function"].(map[string]any)
		if fn == nil {
			continue
		}
		if args, ok := fn["arguments"].(string); ok && args != "" {
			s.fcArgs[idx] += args
			s.emit(map[string]any{"type": "response.function_call_arguments.delta",
				"item_id": fmt.Sprintf("fc_%d", idx), "output_index": s.textIdx + 1 + idx, "delta": args})
		}
		if name, ok := fn["name"].(string); ok && name != "" {
			s.fcName[idx] = name
		}
	}
}

// intFromAny 取 map 里的 index 为 int。
func intFromAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}

// mapString 取嵌套 map 里的字符串值。
func mapString(m map[string]any, keys ...string) (string, bool) {
	var cur any = m
	for _, k := range keys {
		next, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur = next[k]
	}
	s, ok := cur.(string)
	return s, ok
}