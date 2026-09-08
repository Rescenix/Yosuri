package handler

// 并发派发回归测试：子代理一轮内派多个孙代理时，结果收集容器曾被多个 goroutine
// 并发写同一张 map → Go runtime 抛 fatal error: concurrent map writes，
// recover() 拦不住、整个进程崩。修复后改为按下标预分配的 slice。
// 注意：孙代理的 system prompt 里「深度 N」必须是 2（子代理显式以 depth=1 起算）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubAgentConcurrentGrandchildDispatch(t *testing.T) {
	const fanout = 8 // 一轮派 8 个孙代理，足够让并发写 map 稳定炸出来
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []map[string]any `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		sys := ""
		hasTool := false
		for _, m := range req.Messages {
			if m["role"] == "system" {
				sys, _ = m["content"].(string)
			}
			if m["role"] == "tool" {
				hasTool = true
			}
		}
		// 孙代理（深度 2）：直接出结论
		if strings.Contains(sys, "深度 2") {
			w.Write([]byte(`{"choices":[{"message":{"content":"孙代理结论 OK"}}]}`))
			return
		}
		// 子代理（深度 1）：第一轮并发派 fanout 个孙代理，第二轮汇总
		if !hasTool {
			calls := make([]string, 0, fanout)
			for i := 0; i < fanout; i++ {
				calls = append(calls, `{"id":"g`+string(rune('a'+i))+`","type":"function","function":{"name":"dispatch_agent","arguments":"{\"task\":\"调研分片`+string(rune('0'+i))+`\"}"}}`)
			}
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[` + strings.Join(calls, ",") + `]}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"子代理汇总完成"}}]}`))
	}))
	defer srv.Close()

	backends := []RouterBackend{
		{Name: "测试源", BaseURL: srv.URL, Model: "m", Timeout: 15 * time.Second},
	}
	// emit 走真实 writeCodeSSE 之外的裸回调，这里只验证不死；并发安全由内部锁保证。
	var emitMu = make(chan struct{}, 1)
	emitMu <- struct{}{}
	emit := func(string, map[string]any) {
		// 模拟并发回调不崩
		select {
		case <-emitMu:
			emitMu <- struct{}{}
		default:
		}
	}

	out, err := runSubAgent(context.Background(), backends, "root_call", `{"task":"并发调研"}`, emit, 1)
	if err != nil {
		t.Fatalf("并发派孙代理应成功: %v", err)
	}
	if !strings.Contains(out, "子代理汇总完成") {
		t.Fatalf("应拿到子代理汇总, got %q", out)
	}
	t.Logf("✅ %d 个孙代理并发派发无竞争崩溃: %s", fanout, out)
}
