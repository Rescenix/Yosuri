package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func fakeBackend(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

const okCompletion = `{"choices":[{"message":{"content":"来自二号源"}}]}`

func TestRouteChatOnceFailover(t *testing.T) {
	// 一号源 402（余额耗尽），二号源正常 → 应秒切到二号源
	broke := fakeBackend(t, 402, `{"error":{"message":"Insufficient Balance"}}`)
	defer broke.Close()
	good := fakeBackend(t, 200, okCompletion)
	defer good.Close()

	backends := []RouterBackend{
		{Name: "一号(欠费)", BaseURL: broke.URL, Model: "m1", APIKey: "k", Timeout: 5 * time.Second},
		{Name: "二号(正常)", BaseURL: good.URL, Model: "m2", APIKey: "k", Timeout: 5 * time.Second},
	}
	content, _, err := routeChatOnce(context.Background(), backends,
		[]map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil {
		t.Fatalf("failover 应成功: %v", err)
	}
	if content != "来自二号源" {
		t.Fatalf("应由二号源承接, got %q", content)
	}
}

func TestRouteChatOnceAllFail(t *testing.T) {
	broke := fakeBackend(t, 500, `oops`)
	defer broke.Close()
	backends := []RouterBackend{
		{Name: "唯一(挂了)", BaseURL: broke.URL, Model: "m", APIKey: "k", Timeout: 5 * time.Second},
	}
	_, _, err := routeChatOnce(context.Background(), backends,
		[]map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err == nil {
		t.Fatal("全部失败应报错")
	}
}

func TestResolveBackendsAlwaysNonEmpty(t *testing.T) {
	backends := resolveBackends("nonexistent_user_for_test", "")
	if len(backends) == 0 {
		t.Fatal("链不应为空")
	}
	last := backends[len(backends)-1]
	if last.Name == "" || last.BaseURL == "" {
		t.Fatalf("链尾 backend 不应为空, got %+v", last)
	}
}

func TestResolveBackendsExactFreeModel(t *testing.T) {
	// 给定免费池里一个有环境变量的模型 ID，应精确返回单 backend 且带能力元数据
	b := resolveExact("nonexistent_user_for_test", "free_google_gemini_2_5_flash")
	if b == nil {
		t.Skip("无 GOOGLE_API_KEY 环境变量，跳过精确命中断言")
	}
	if b.Source != "free" || b.Model != "gemini-2.5-flash" {
		t.Fatalf("精确命中应返回对应免费 backend, got %+v", b)
	}
	if !b.Vision || b.ContextWindow != 1048576 || !b.Reasoning {
		t.Fatalf("能力元数据应随 backend 透出, got %+v", b)
	}
	// 回退路径：未知 model 应回退到含本地尾的全链
	fallback := resolveBackends("nonexistent_user_for_test", "nonexistent_id")
	if len(fallback) == 0 || !fallback[len(fallback)-1].IsLocal {
		t.Fatalf("未知 model 应回退全链且本地兜底, got %+v", fallback)
	}
}
