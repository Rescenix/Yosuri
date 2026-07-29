package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func hasNativeTool(name string) bool {
	for _, def := range nativeOnDemandToolDefs() {
		if def.Function.Name == name {
			return true
		}
	}
	return false
}

func TestNativeWebSearchRegistrationRequiresKey(t *testing.T) {
	t.Setenv("BING_SEARCH_API_KEY", "")
	if hasNativeTool("web_search") {
		t.Fatal("无 BING_SEARCH_API_KEY 时不应注册 web_search")
	}
	if !hasNativeTool("web_fetch") {
		t.Fatal("无 Key 时仍应保留免 Key 的 web_fetch")
	}

	t.Setenv("BING_SEARCH_API_KEY", "configured")
	if !hasNativeTool("web_search") {
		t.Fatal("配置 BING_SEARCH_API_KEY 后应注册 web_search")
	}
}

func TestNativeWebSearchUsesGoHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Ocp-Apim-Subscription-Key"); got != "test-key" {
			t.Errorf("搜索 Key 头不正确: %q", got)
		}
		if got := r.URL.Query().Get("q"); got != "Go MCP" {
			t.Errorf("搜索词不正确: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"webPages":{"value":[{"name":"Result","url":"https://example.com/doc","snippet":"Go native search"}]}}`))
	}))
	defer server.Close()

	t.Setenv("BING_SEARCH_API_KEY", "test-key")
	t.Setenv("BING_SEARCH_ENDPOINT", server.URL)
	result, err := callNativeTool(context.Background(), "web_search", `{"query":"Go MCP","count":3}`)
	if err != nil {
		t.Fatalf("Go web_search 失败: %v", err)
	}
	for _, want := range []string{"1. Result", "https://example.com/doc", "Go native search"} {
		if !strings.Contains(result.Text, want) {
			t.Errorf("搜索结果缺少 %q: %s", want, result.Text)
		}
	}
}
