package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRemoteMCPStreamableHTTPHandshakeAndSSE(t *testing.T) {
	var sawSession atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			t.Errorf("请求不是 JSON-RPC: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if rpc.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "session-1")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18"}}`, rpc.ID)
			return
		}
		if r.Header.Get("Mcp-Session-Id") == "session-1" {
			sawSession.Store(true)
		}
		if rpc.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"tools\":[{\"name\":\"ping\",\"description\":\"Ping\",\"inputSchema\":{\"type\":\"object\",\"properties\":{}}}]}}\n\n", rpc.ID)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done() // 模拟保持打开的 SSE；客户端拿到首个事件后必须立即返回
	}))
	defer server.Close()

	conn, defs, err := startRemoteMCPServer("remote-test", mcpServerConfig{URL: server.URL})
	if err != nil {
		t.Fatalf("远程 MCP 启动失败: %v", err)
	}
	defer conn.close()
	if len(defs) != 1 || defs[0].Function.Name != "ping" {
		t.Fatalf("远程 tools/list 未转换: %+v", defs)
	}
	if !sawSession.Load() {
		t.Fatal("后续请求未携带 MCP session id")
	}
}

func TestOfficialMCPRegistryOnlyReturnsDirectHTTPServers(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dataDir)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("version"); got != "latest" {
			t.Errorf("version=%q，期望 latest", got)
		}
		_, _ = w.Write([]byte(`{"servers":[
			{"server":{"name":"io.test/remote","title":"Remote","description":"works","version":"1.0.0","remotes":[{"type":"streamable-http","url":"https://example.com/mcp"}]}},
			{"server":{"name":"io.test/auth","title":"Auth","version":"1.0.0","remotes":[{"type":"streamable-http","url":"https://example.com/auth","headers":[{"name":"Authorization","isRequired":true}]}]}},
			{"server":{"name":"io.test/local","title":"Local","version":"1.0.0","packages":[{"registryType":"npm"}]}}
		],"metadata":{"count":3}}`))
	}))
	defer server.Close()
	old := officialMCPRegistryBaseURL
	officialMCPRegistryBaseURL = server.URL
	t.Cleanup(func() { officialMCPRegistryBaseURL = old })

	items, err := fetchOfficialMCPRegistry("", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "io.test/remote" || items[0].URL != "https://example.com/mcp" {
		t.Fatalf("Registry 过滤结果错误: %+v", items)
	}
}

func TestInstallHostedSkillFromGitHubAPI(t *testing.T) {
	extDir := t.TempDir()
	t.Setenv("AURORA_EXT_SKILLS_DIR", extDir)
	skillMD := "---\nname: demo-skill\ndescription: demo\n---\n\n# Demo"
	files := map[string]string{
		"/anthropics/skills/main/skills/demo-skill/SKILL.md":            skillMD,
		"/anthropics/skills/main/skills/demo-skill/references/guide.md": "reference text",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/anthropics/skills":
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case r.URL.Path == "/repos/anthropics/skills/git/trees/main":
			_, _ = w.Write([]byte(`{"tree":[
				{"path":"skills/demo-skill/SKILL.md","type":"blob","sha":"sha-skill","size":64},
				{"path":"skills/demo-skill/references/guide.md","type":"blob","sha":"sha-ref","size":14}
			],"truncated":false}`))
		case strings.HasPrefix(r.URL.Path, "/anthropics/skills/main/"):
			content, ok := files[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(content))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	oldBase := githubAPIBaseURL
	oldRawBase := githubRawBaseURL
	githubAPIBaseURL = server.URL
	githubRawBaseURL = server.URL
	githubSkillTreeMu.Lock()
	githubSkillTreeCache = map[string]cachedGitHubSkillTree{}
	githubSkillTreeMu.Unlock()
	t.Cleanup(func() {
		githubAPIBaseURL = oldBase
		githubRawBaseURL = oldRawBase
	})

	gin.SetMode(gin.TestMode)
	ginRouter := gin.New()
	ginRouter.POST("/install", HandleInstallHostedSkill)
	req := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(`{
		"source":"anthropics/skills","path":"skills/demo-skill/SKILL.md"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ginRouter.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("安装返回 %d: %s", rec.Code, rec.Body.String())
	}
	if data, err := os.ReadFile(filepath.Join(extDir, "demo-skill", "SKILL.md")); err != nil || string(data) != skillMD {
		t.Fatalf("SKILL.md 未正确落盘: err=%v content=%q", err, data)
	}
	if _, err := os.Stat(filepath.Join(extDir, "demo-skill", "references", "guide.md")); err != nil {
		t.Fatalf("附属文件未安装: %v", err)
	}
	loaded := loadExternalSkills()
	if len(loaded) != 1 || loaded[0].Name != "demo-skill" || loaded[0].Provider != "anthropics/skills" {
		t.Fatalf("安装后未进入技能库: %+v", loaded)
	}
}

func TestSafeRelativeSkillPathRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"", "../secret", "a/../../secret", `a\..\secret`, "/absolute"} {
		if safeRelativeSkillPath(bad) {
			t.Errorf("危险路径被放行: %q", bad)
		}
	}
	for _, good := range []string{"SKILL.md", "references/guide.md", "scripts/check.go"} {
		if !safeRelativeSkillPath(good) {
			t.Errorf("合法路径被拒绝: %q", good)
		}
	}
}
