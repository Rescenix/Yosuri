package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 孙代理递归派发测试（2026-08-16 加，对齐 Codex 嵌套能力）
// 模拟链路：子代理(深度1) → dispatch_agent 派孙代理(深度2) → 孙代理直接出结论 → 子代理汇总
func TestSubAgentGrandchildDispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		sys := ""
		for _, m := range req.Messages {
			if m["role"] == "system" {
				sys, _ = m["content"].(string)
			}
		}
		// 孙代理（深度 2）：直接出结论，不再派发
		if strings.Contains(sys, "深度 2") {
			w.Write([]byte(`{"choices":[{"message":{"content":"孙代理结论：cli.js 共 42 行"}}]}`))
			return
		}
		// 子代理（深度 1）：第一轮派孙代理，第二轮拿到结果后汇总
		hasTool := false
		for _, m := range req.Messages {
			if m["role"] == "tool" {
				hasTool = true
			}
		}
		if !hasTool {
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"sub1","type":"function","function":{"name":"dispatch_agent","arguments":"{\"task\":\"调研 cli.js 结构\"}"}}]}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"子代理汇总：孙代理已读完 cli.js"}}]}`))
	}))
	defer srv.Close()

	backends := []RouterBackend{
		{Name: "测试源", BaseURL: srv.URL, Model: "m", APIKey: "k", Timeout: 10 * time.Second},
	}
	var events []string
	emit := func(typ string, data map[string]any) {
		events = append(events, typ+":"+stringifyDepth(data))
	}

	out, err := runSubAgent(context.Background(), backends, "root_call", `{"task":"读 cli.js"}`, emit, 1)
	if err != nil {
		t.Fatalf("子代理跑孙代理应成功: %v", err)
	}
	if !strings.Contains(out, "孙代理已读完") {
		t.Fatalf("子代理应汇总孙代理结论, got %q", out)
	}
	// 事件里应同时出现 depth=1 和 depth=2 的 subagent_start
	seen1, seen2 := false, false
	for _, e := range events {
		if strings.Contains(e, "subagent_start") && strings.Contains(e, "depth:1") {
			seen1 = true
		}
		if strings.Contains(e, "subagent_start") && strings.Contains(e, "depth:2") {
			seen2 = true
		}
	}
	if !seen1 || !seen2 {
		t.Fatalf("应同时派发子(深度1)+孙(深度2), events=%v", events)
	}
	t.Logf("✅ 孙代理链路打通: %s", out)
}

// 深度上限：深度 3 的子代理再派 dispatch_agent → 返回上限提示，不递归
func TestSubAgentDepthLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		hasTool := false
		for _, m := range req.Messages {
			if m["role"] == "tool" {
				hasTool = true
			}
		}
		// 收到工具结果（深度上限提示）后结束本轮，并转述工具结果以便断言
		if hasTool {
			var toolMsg string
			for _, m := range req.Messages {
				if m["role"] == "tool" {
					toolMsg, _ = m["content"].(string)
					break
				}
			}
			w.Write([]byte(`{"choices":[{"message":{"content":"` + toolMsg + `"}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"deep1","type":"function","function":{"name":"dispatch_agent","arguments":"{\"task\":\"再派一层\"}"}}]}}]}`))
	}))
	defer srv.Close()

	backends := []RouterBackend{
		{Name: "测试源", BaseURL: srv.URL, Model: "m", APIKey: "k", Timeout: 10 * time.Second},
	}
	out, err := runSubAgent(context.Background(), backends, "root_call", `{"task":"深挖"}`, nil, subAgentMaxDepth)
	if err != nil {
		t.Fatalf("深度上限应返回提示而非报错: %v", err)
	}
	if !strings.Contains(out, "嵌套深度已达上限") {
		t.Fatalf("应提示深度上限, got %q", out)
	}
	t.Logf("✅ 深度上限生效: %s", out)
}

func stringifyDepth(data map[string]any) string {
	if v, ok := data["depth"]; ok {
		return "depth:" + stringifyAny(v)
	}
	return "depth:-"
}

func stringifyAny(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
