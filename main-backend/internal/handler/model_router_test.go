package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

// 本地兜底路由已在 8186699e 移除，「链永不为空」不再成立：一个 Key 都没配时
// 链就是空的（这个用例原来能过，只是因为 Ollama Cloud 条目当时允许无 Key 入链，
// 而那种条目一发请求必然 401——空链伪装成非空，反而掩盖了"没配 Key"这个真实问题）。
// 现在的契约改成：链里出现的每一个 backend 都必须是可用的（有名字、有地址、有 Key）。
func TestResolveBackendsChainEntriesAreUsable(t *testing.T) {
	backends := resolveBackends("nonexistent_user_for_test", "")
	if len(backends) == 0 {
		t.Skip("当前环境没有任何 API Key，空链是预期结果（错误提示由 streamRouterRound 给出）")
	}
	for _, b := range backends {
		if b.Name == "" || b.BaseURL == "" {
			t.Errorf("链里有残缺 backend: %+v", b)
		}
		// 非本地、非免 key 网关的 backend 无 Key 却入了链 → 发出去必然 401
		if b.APIKey == "" && !b.IsLocal && !b.Keyless {
			t.Errorf("非本地/非免key backend 无 Key 却入了链，发出去必然 401: %s", b.Name)
		}
	}
}

// 空链必须给出能指导用户的错误，而不是 "所有模型源不可用：" 后面一片空白。
func TestStreamRouterRoundEmptyChainMessage(t *testing.T) {
	r := &WorkflowRunner{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/code/workflow", nil)
	_, _, _, _, _, err := r.streamRouterRound(c, nil, nil, nil, "", 0)
	if err == nil {
		t.Fatal("空链应报错")
	}
	if !strings.Contains(err.Error(), "API Key") {
		t.Errorf("错误信息应告诉用户去配 Key，实得: %v", err)
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
	// 回退路径：未知 model 不该返回空，应回退到全链（本地兜底已移除，故不再断言链尾 IsLocal）
	fallback := resolveBackends("nonexistent_user_for_test", "nonexistent_id")
	if len(fallback) == 0 {
		t.Fatal("未知 model 应回退全链, 实得空链")
	}
}
