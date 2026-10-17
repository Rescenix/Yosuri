package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"backend/internal/ai/core"
	"github.com/gin-gonic/gin"
)

type parallelRoundTrip func(*http.Request) (*http.Response, error)

func (f parallelRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func parallelTestTransport(t *testing.T, f parallelRoundTrip) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = f
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func parallelTestConfig(t *testing.T, entries []ModelConfigEntry) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // isolate saved credentials as well as environment keys
	t.Setenv("FIRECRAWL_API_KEY", "")
	t.Setenv("PARALLEL_API_KEY", "")
	if err := saveModelConfigs("", entries); err != nil {
		t.Fatal(err)
	}
}

func parallelTestResponse(r *http.Request, result any, sse bool) *http.Response {
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	typ := "application/json"
	if sse {
		data = []byte("event: message\ndata: " + string(data) + "\n\n")
		typ = "text/event-stream"
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {typ}, "Mcp-Session-Id": {"fixture-session"}}, Body: io.NopCloser(strings.NewReader(string(data))), Request: r}
}

// This exercises saved selection -> workflow dispatcher -> native web_search ->
// MCP request -> model text AND the citations consumed by the frontend.
func TestParallelWorkflowSearch(t *testing.T) {
	for _, sse := range []bool{false, true} {
		t.Run(map[bool]string{false: "JSON", true: "SSE"}[sse], func(t *testing.T) {
			parallelTestConfig(t, []ModelConfigEntry{{ID: "websearch", Extra: map[string]string{"mode": "parallel"}}})
			t.Setenv("PARALLEL_API_KEY", "must-not-leak")
			var methods []string
			var session string
			parallelTestTransport(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != parallelSearchURL || r.Method != "POST" {
					t.Errorf("unexpected destination %s %s", r.Method, r.URL)
				}
				if r.Header.Get("User-Agent") != parallelSearchUserAgent || r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Key") != "" {
					t.Errorf("unexpected headers %v", r.Header)
				}
				if r.Header.Get("Accept") != "application/json, text/event-stream" {
					t.Error("missing MCP Accept")
				}
				var rpc struct {
					Method string
					Params struct {
						Name      string
						Arguments map[string]any
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
					t.Fatal(err)
				}
				methods = append(methods, rpc.Method)
				if len(methods) > 1 && r.Header.Get("Mcp-Session-Id") != "fixture-session" {
					t.Error("lost MCP session")
				}
				switch rpc.Method {
				case "initialize", "notifications/initialized":
					return parallelTestResponse(r, map[string]any{}, sse), nil
				case "tools/list":
					return parallelTestResponse(r, map[string]any{"tools": []any{map[string]any{"name": "web_search"}}}, sse), nil
				case "tools/call":
					if rpc.Params.Name != "web_search" || rpc.Params.Arguments["objective"] != "Go documentation" || rpc.Params.Arguments["query"] != nil {
						t.Errorf("bad MCP arguments %+v", rpc.Params)
					}
					queries, _ := rpc.Params.Arguments["search_queries"].([]any)
					if len(queries) != 1 || queries[0] != "Go documentation" {
						t.Error("missing search_queries")
					}
					id, _ := rpc.Params.Arguments["session_id"].(string)
					if len(id) != 64 || (session != "" && id != session) {
						t.Error("unstable anonymous session")
					}
					session = id
					return parallelTestResponse(r, map[string]any{"structuredContent": map[string]any{"results": []any{
						map[string]any{"url": "javascript:alert(1)", "title": "unsafe"},
						map[string]any{"url": "https://go.dev/doc/", "title": "Go docs", "excerpts": []string{"Learn Go"}},
						map[string]any{"url": "https://go.dev/tour/", "title": "Tour"},
					}}}, sse), nil
				}
				t.Fatalf("unexpected MCP method %s", rpc.Method)
				return nil, nil
			})
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/", nil)
			call := core.ToolCall{ID: "search", Function: core.ToolCallFunc{Name: "web_search", Arguments: `{"query":"Go documentation","max_results":1}`}}
			for i := 0; i < 2; i++ {
				methods = nil
				results := (&WorkflowRunner{}).executeCodeCalls(c, nil, []core.ToolCall{call}, func(string, map[string]any) {}, "ask", nil, "chat-one", "workflow-one")
				if len(results) != 1 || results[0].failed || !strings.Contains(results[0].output, "Learn Go") || len(results[0].urls) != 1 || results[0].urls[0] != "https://go.dev/doc/" {
					t.Fatalf("workflow result %+v", results)
				}
				if strings.Join(methods, ",") != "initialize,notifications/initialized,tools/list,tools/call" {
					t.Fatalf("MCP lifecycle %v", methods)
				}
			}
		})
	}
}

func TestParallelErrorsDoNotFallback(t *testing.T) {
	for _, scenario := range []string{"http", "rpc", "tool", "malformed", "missing", "invalid-results", "no-tool", "cancel", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			parallelTestConfig(t, []ModelConfigEntry{{ID: "websearch", Extra: map[string]string{"mode": "parallel"}}})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "timeout" {
				ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
			}
			parallelTestTransport(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != parallelSearchURL {
					t.Fatalf("silently fell back to %s", r.URL)
				}
				var rpc struct{ Method string }
				json.NewDecoder(r.Body).Decode(&rpc)
				if rpc.Method == "tools/list" {
					if scenario == "no-tool" {
						return parallelTestResponse(r, map[string]any{"tools": []any{}}, false), nil
					}
					return parallelTestResponse(r, map[string]any{"tools": []any{map[string]any{"name": "web_search"}}}, false), nil
				}
				if rpc.Method != "tools/call" {
					return parallelTestResponse(r, map[string]any{}, false), nil
				}
				if scenario == "cancel" {
					cancel()
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				if scenario == "timeout" {
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				resp := parallelTestResponse(r, map[string]any{}, false)
				switch scenario {
				case "http":
					resp.StatusCode = 429
					resp.Body = io.NopCloser(strings.NewReader("quota exhausted"))
				case "rpc":
					resp.Body = io.NopCloser(strings.NewReader(`{"error":{"code":-1,"message":"search failed"}}`))
				case "tool":
					return parallelTestResponse(r, map[string]any{"isError": true, "content": []any{map[string]string{"type": "text", "text": "quota exhausted"}}}, false), nil
				case "malformed":
					resp.Body = io.NopCloser(strings.NewReader("not JSON"))
				case "invalid-results":
					return parallelTestResponse(r, map[string]any{"structuredContent": map[string]any{"results": []any{map[string]any{"url": 3}}}}, false), nil
				}
				return resp, nil
			})
			_, err := callNativeTool(ctx, "web_search", `{"query":"Go documentation"}`)
			if err == nil {
				t.Fatal("expected provider error")
			}
			if scenario == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if scenario == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
		})
	}
}

func TestParallelPreservesSearchChoices(t *testing.T) {
	for _, mode := range []string{"", "bing", "firecrawl", "custom", "mcp"} {
		t.Run("mode="+mode, func(t *testing.T) {
			var entries []ModelConfigEntry
			if mode != "" {
				entries = []ModelConfigEntry{{ID: "websearch", Endpoint: "https://custom.example", APIKey: "saved-key", DefaultModel: "saved-model", Extra: map[string]string{"mode": mode}}}
				if mode == "firecrawl" {
					entries = append(entries, ModelConfigEntry{ID: "firecrawl", APIKey: "saved-firecrawl-key"})
				}
			}
			parallelTestConfig(t, entries)
			var destination string
			parallelTestTransport(t, func(r *http.Request) (*http.Response, error) {
				destination = r.URL.Host
				if r.URL.String() == parallelSearchURL {
					t.Fatal("Parallel replaced an existing choice")
				}
				if mode == "custom" && r.Header.Get("Authorization") != "Bearer saved-key" {
					t.Fatal("custom key lost")
				}
				if mode == "firecrawl" && r.Header.Get("Authorization") != "Bearer saved-firecrawl-key" {
					t.Fatal("Firecrawl key lost")
				}
				return nil, errors.New("fixture stops existing provider")
			})
			callNativeTool(context.Background(), "web_search", `{"query":"Go documentation"}`)
			want := map[string]string{"": "cn.bing.com", "bing": "cn.bing.com", "firecrawl": "api.firecrawl.dev", "custom": "custom.example", "mcp": ""}[mode]
			if destination != want {
				t.Fatalf("destination %s want %s", destination, want)
			}
			after, err := loadModelConfigs("")
			if err != nil {
				t.Fatal(err)
			}
			beforeJSON, _ := json.Marshal(entries)
			afterJSON, _ := json.Marshal(after)
			if string(beforeJSON) != string(afterJSON) {
				t.Fatal("search changed saved settings")
			}
		})
	}
}

func TestParallelTextContentAndLimits(t *testing.T) {
	var items []any
	for i := 0; i < 12; i++ {
		items = append(items, map[string]any{"url": "https://go.dev/doc/", "title": "Go"})
	}
	data, _ := json.Marshal(map[string]any{"results": items})
	raw, _ := json.Marshal(map[string]any{"content": []any{map[string]string{"type": "text", "text": string(data)}}})
	for _, limit := range []int{0, 1, 10, 20} {
		_, urls, err := formatParallelSearch(raw, "Go", limit)
		want := limit
		if want <= 0 {
			want = 5
		}
		if want > 10 {
			want = 10
		}
		if err != nil || len(urls) != want {
			t.Fatalf("limit %d: %v %v", limit, urls, err)
		}
	}
}

// Explicit opt-in only. Use the same saved selection and workflow path as a user.
func TestParallelLiveWorkflow(t *testing.T) {
	if os.Getenv("YOSURI_PARALLEL_LIVE") != "1" {
		t.Skip("set YOSURI_PARALLEL_LIVE=1 for anonymous live search")
	}
	parallelTestConfig(t, []ModelConfigEntry{{ID: "websearch", Extra: map[string]string{"mode": "parallel"}}})
	transport := http.DefaultTransport
	parallelTestTransport(t, func(r *http.Request) (*http.Response, error) {
		copy, _ := r.GetBody()
		var rpc struct{ Method string }
		if copy != nil {
			json.NewDecoder(copy).Decode(&rpc)
			copy.Close()
		}
		t.Logf("request %s %s User-Agent=%s Authorization-present=%t", rpc.Method, r.URL, r.Header.Get("User-Agent"), r.Header.Get("Authorization") != "")
		resp, err := transport.RoundTrip(r)
		if err == nil && rpc.Method == "tools/call" {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			t.Logf("live response at %s: status=%d headers=%v body=%s", time.Now().UTC().Format(time.RFC3339), resp.StatusCode, resp.Header, body)
			resp.Body = io.NopCloser(strings.NewReader(string(body)))
		}
		return resp, err
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	call := core.ToolCall{ID: "search", Function: core.ToolCallFunc{Name: "web_search", Arguments: `{"query":"Go programming language official documentation","max_results":3}`}}
	result := (&WorkflowRunner{}).executeCodeCalls(c, nil, []core.ToolCall{call}, func(string, map[string]any) {}, "ask", nil, "yosuri-parallel-live", "live")
	if result[0].failed {
		t.Fatal(result[0].output)
	}
	if len(result[0].urls) == 0 {
		t.Fatal("live search returned no citations")
	}
	t.Logf("actual workflow output: %s; citations: %v", result[0].output, result[0].urls)
}
