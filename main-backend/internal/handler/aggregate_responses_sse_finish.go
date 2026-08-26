package handler

// aggregate_responses_sse_finish.go —— Responses SSE 流式收尾事件（done + completed）。

import "fmt"

// finish 关闭流式响应，依次发出：
//   - output_text.done / content_part.done / output_item.done（message）
//   - function_call_arguments.done / output_item.done（每个 function_call）
//   - response.completed
func (s *responsesSSEState) finish() {
	// message 收尾
	if s.textStarted {
		text := s.text.String()
		s.emit(map[string]any{"type": "response.output_text.done", "item_id": s.msgID,
			"output_index": 0, "content_index": 0, "text": text})
		s.emit(map[string]any{"type": "response.content_part.done", "item_id": s.msgID,
			"output_index": 0, "content_index": 0,
			"part": map[string]any{"type": "output_text", "text": text, "annotations": []any{}}})
		s.emit(map[string]any{"type": "response.output_item.done", "output_index": 0, "item": map[string]any{
			"id": s.msgID, "type": "message", "status": "completed", "role": "assistant",
			"content": []map[string]any{{"type": "output_text", "text": text, "annotations": []any{}}},
		}})
		s.output = append(s.output, map[string]any{
			"type": "message", "id": s.msgID, "status": "completed", "role": "assistant",
			"content": []map[string]any{{"type": "output_text", "text": text, "annotations": []any{}}},
		})
	}
	// 每个 function_call 收尾
	for idx := range s.fcSeen {
		args := s.fcArgs[idx]
		id := fmt.Sprintf("fc_%d", idx)
		s.emit(map[string]any{"type": "response.function_call_arguments.done",
			"item_id": id, "output_index": s.textIdx + 1 + idx, "arguments": args})
		s.emit(map[string]any{"type": "response.output_item.done", "output_index": s.textIdx + 1 + idx, "item": map[string]any{
			"id": id, "type": "function_call", "status": "completed",
			"call_id": s.fcCallID[idx], "name": s.fcName[idx], "arguments": args,
		}})
		s.output = append(s.output, map[string]any{
			"type": "function_call", "id": id, "status": "completed",
			"call_id": s.fcCallID[idx], "name": s.fcName[idx], "arguments": args,
		})
	}
	s.emit(map[string]any{"type": "response.completed", "response": s.respMeta("completed")})
}