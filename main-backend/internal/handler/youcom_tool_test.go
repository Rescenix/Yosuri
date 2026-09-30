package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// youcomFakeMCP 起一个只服务 youcomSearch 握手流程的假 You.com MCP server：
// initialize → JSON 应答；notifications/initialized → 202；tools/call → SSE
// （先推一条 notifications/message 进展通知，再推真正的结果——与线上实测一致）。
func youcomFakeMCP(t *testing.T, resultText string, callArgs *map[string]any, sawAuth func(bool)) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sawAuth != nil {
			sawAuth(r.Header.Get("Authorization") != "")
		}
		var rpc struct {
			ID     *int64         `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			t.Errorf("请求不是 JSON-RPC: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch rpc.Method {
		case "initialize":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"You.com","version":"test"}}}`, *rpc.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			mu.Lock()
			if callArgs != nil {
				*callArgs = rpc.Params
			}
			mu.Unlock()
			note, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/message", "params": map[string]any{"level": "info", "data": "Search successful"}})
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", note)
			msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *rpc.ID, "result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": resultText}},
				"isError": false,
			}})
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
		default:
			t.Errorf("未预期的 MCP 方法: %s", rpc.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// youcomSwapURLs 把免 key / 认证端点都指到测试 server（结束后还原）。
func youcomSwapURLs(t *testing.T, freeURL, authURL string) {
	t.Helper()
	oldFree, oldAuth := youcomFreeMCPURL, youcomAuthenticatedURL
	youcomFreeMCPURL, youcomAuthenticatedURL = freeURL, authURL
	t.Cleanup(func() { youcomFreeMCPURL, youcomAuthenticatedURL = oldFree, oldAuth })
}

func TestYoucomSearchSSEFlow(t *testing.T) {
	t.Setenv("YDC_API_KEY", "")
	resultJSON, _ := json.Marshal(map[string]any{"results": map[string]any{"web": []map[string]any{
		{"url": "https://example.com/a", "title": "结果甲", "description": "摘要甲"},
		{"url": "https://example.com/b", "title": "结果乙", "description": ""},
	}}})
	var callArgs map[string]any
	var sawAuthHeader bool
	server := youcomFakeMCP(t, string(resultJSON), &callArgs, func(ok bool) { sawAuthHeader = ok })
	youcomSwapURLs(t, server.URL, server.URL)

	text, urls, err := youcomSearch(context.Background(), "测试查询", 50)
	if err != nil {
		t.Fatalf("youcomSearch 失败: %v", err)
	}
	// 免 key 默认：请求不应带 Authorization
	if sawAuthHeader {
		t.Error("未配置 YDC_API_KEY 时不应携带 Authorization 头")
	}
	// 工具名与参数：you-search，query 原样带回，count 被夹到上限 10
	if callArgs["name"] != "you-search" {
		t.Fatalf("应调用 you-search，实际 %v", callArgs["name"])
	}
	args, _ := callArgs["arguments"].(map[string]any)
	if args["query"] != "测试查询" {
		t.Errorf("query 未透传: %v", args["query"])
	}
	if args["count"] != float64(10) {
		t.Errorf("count 应夹到 10，实际 %v", args["count"])
	}
	// 文本格式与来源 URL
	for _, want := range []string{"联网搜索「测试查询」结果（You.com）：", "1. 结果甲", "https://example.com/a", "2. 结果乙"} {
		if !strings.Contains(text, want) {
			t.Errorf("输出缺少 %q:\n%s", want, text)
		}
	}
	if len(urls) != 2 || urls[0] != "https://example.com/a" || urls[1] != "https://example.com/b" {
		t.Errorf("来源 URL 列表错误: %v", urls)
	}
}

func TestYoucomSearchAuthenticatedUsesBearerAndEndpoint(t *testing.T) {
	t.Setenv("YDC_API_KEY", "test-key-123")
	// 免 key 端点被指到一个「不该被碰」的 server；认证端点承担全部请求
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("配置了 YDC_API_KEY 仍访问了免 key 端点")
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(blocked.Close)

	var sawAuthHeader bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key-123" {
			t.Errorf("认证请求应带 Bearer 头，实际 %q", got)
		}
		sawAuthHeader = true
		var rpc struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch rpc.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18"}}`, *rpc.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		default:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"{\"results\":{\"web\":[]}}"}],"isError":false}}`, *rpc.ID)
		}
	}))
	t.Cleanup(server.Close)
	youcomSwapURLs(t, blocked.URL, server.URL)

	text, _, err := youcomSearch(context.Background(), "q", 5)
	if err != nil {
		t.Fatalf("认证端点搜索失败: %v", err)
	}
	if !sawAuthHeader {
		t.Error("未发出认证请求")
	}
	if !strings.Contains(text, "没有找到结果") {
		t.Errorf("空结果应给友好提示，实际: %s", text)
	}
}

func TestYoucomSearchHTTPErrorPropagates(t *testing.T) {
	t.Setenv("YDC_API_KEY", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if rpc.Method == "tools/call" {
			http.Error(w, "upstream exploded", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18"}}`)
	}))
	t.Cleanup(server.Close)
	youcomSwapURLs(t, server.URL, server.URL)

	if _, _, err := youcomSearch(context.Background(), "q", 5); err == nil {
		t.Fatal("tools/call 5xx 应返回错误，而不是静默吞掉")
	}
}

func TestYoucomReadSSEMessagesMultiLineData(t *testing.T) {
	stream := "event: message\ndata: {\"jsonrpc\":\"2.0\",\ndata: \"method\":\"notifications/message\"}\n\ndata: not-json-should-be-skipped\n\n"
	msgs, err := youcomReadSSEMessages(strings.NewReader(stream), 1<<20)
	if err != nil {
		t.Fatalf("SSE 解析失败: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("应收集到 2 条消息，实际 %d", len(msgs))
	}
	var first struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(msgs[0], &first); err != nil || first.Method != "notifications/message" {
		t.Errorf("多行 data 拼接错误: %s (err=%v)", msgs[0], err)
	}
}
