package handler

// 预算闸门单元测试：验证 subAgentHistoryChars 正确累计 content + tool_calls 参数，
// 这是 runSubAgent 里「历史膨胀超 subAgentBudgetChars 就强制收敛」闸门的判定依据。

import (
	"strings"
	"testing"
)

func TestSubAgentHistoryChars(t *testing.T) {
	msgs := []map[string]any{
		{"role": "system", "content": strings.Repeat("a", 100)},
		{"role": "user", "content": strings.Repeat("b", 200)},
		{"role": "assistant", "content": strings.Repeat("c", 50), "tool_calls": []map[string]any{
			{"function": map[string]any{"name": "grep", "arguments": strings.Repeat("d", 300)}},
			{"function": map[string]any{"name": "read_file", "arguments": strings.Repeat("e", 150)}},
		}},
		{"role": "tool", "content": strings.Repeat("f", 400)},
	}
	got := subAgentHistoryChars(msgs)
	// 100+200+50+300+150+400 = 1200
	want := 1200
	if got != want {
		t.Fatalf("subAgentHistoryChars = %d, 期望 %d", got, want)
	}
	t.Logf("✅ 历史字符量计算正确: %d", got)
}

func TestSubAgentHistoryCharsEmpty(t *testing.T) {
	if n := subAgentHistoryChars(nil); n != 0 {
		t.Fatalf("空历史应为 0, got %d", n)
	}
	// content 非字符串、tool_calls 结构异常都不该 panic
	msgs := []map[string]any{
		{"role": "user", "content": nil},
		{"role": "assistant", "tool_calls": "not-a-slice"},
	}
	if n := subAgentHistoryChars(msgs); n != 0 {
		t.Fatalf("异常结构应安全返回 0, got %d", n)
	}
}
