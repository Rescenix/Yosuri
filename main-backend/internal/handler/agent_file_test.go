package handler

// agent_file_test.go —— 文件交付链路验证：
//   1. deliverableFromPath 只认可交付扩展名，源码/配置不弹卡片；
//   2. HandleAgentFile 端点按扩展名分类 + 文本回读 + raw 下载。
// 不依赖运行中的 8080 服务，纯内存 + 临时目录，可重复。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"backend/internal/ai/core"
)

// isolateTestProjectRoot 把工作目录切到临时目录，并用环境变量重定向状态文件，
// 防止 SetProjectRoot 落盘改掉真实 agent 的工作目录。测试结束恢复原值。
func isolateTestProjectRoot(t *testing.T) {
	t.Helper()
	t.Setenv("SHANXI_WORKDIR_STATE_FILE", filepath.Join(t.TempDir(), "workdir.txt"))
	original := core.GetProjectRoot()
	if err := core.SetProjectRoot(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.SetProjectRoot(original) })
}

func TestDeliverableFromPath(t *testing.T) {
	isolateTestProjectRoot(t)
	root := core.GetProjectRoot()

	// 可交付扩展名应命中
	for _, ext := range []string{".md", ".pdf", ".pptx", ".docx", ".xlsx", ".html", ".txt"} {
		p := filepath.Join(root, "deliver"+ext)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		files := deliverableFromPath(p)
		if len(files) != 1 {
			t.Fatalf("%s 应被视为可交付文件，得到 %d", ext, len(files))
		}
		if files[0].Ext != ext {
			t.Errorf("%s ext 应为 %s，得到 %s", ext, ext, files[0].Ext)
		}
	}

	// 源码/配置不应命中（路径是相对 root 的）
	for _, ext := range []string{".go", ".py", ".js", ".json", ".yaml", ".vue"} {
		p := filepath.Join(root, "code"+ext)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if files := deliverableFromPath(p); len(files) != 0 {
			t.Errorf("%s 不应作为交付物，得到 %d 个", ext, len(files))
		}
	}
}

func TestHandleAgentFile_ServeAndRaw(t *testing.T) {
	isolateTestProjectRoot(t)
	root := core.GetProjectRoot()

	content := "# 测试\n\n```mermaid\ngraph LR\nA-->B\n```\n"
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/agent/file", HandleAgentFile)

	// 1) serve（分类 + 文本回读）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/file?path=doc.md", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("serve 应 200，得到 %d: %s", w.Code, w.Body.String())
	}
	var meta struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Size    int64  `json:"size"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Name != "doc.md" || meta.Kind != "text" || meta.Content != content {
		t.Errorf("serve 元数据不符: %+v", meta)
	}

	// 2) raw 下载
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/agent/file?path=doc.md&raw=1", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("raw 应 200，得到 %d", w2.Code)
	}
	if w2.Body.String() != content {
		t.Error("raw 内容不一致")
	}

	// 3) 越界/不存在应拒绝
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/agent/file?path=../outside.md", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code == http.StatusOK {
		t.Error("越界路径不应 200")
	}
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/api/agent/file?path=nope.md", nil)
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusNotFound {
		t.Errorf("不存在应 404，得到 %d", w4.Code)
	}
}

// TestHandleAgentFile_UserDataAbsolute 覆盖交付卡片绝对路径（媒体/记忆目录都在
// 工作目录外）。09-10 定稿语义：闸门只跟「受保护工作区」开关挂钩——普通模式
// 全放行（卡片路径是服务端自己推的，不是用户输入），凭据文件任何模式都拒，
// 受保护模式才要求审批注册表放行。
func TestHandleAgentFile_UserDataAbsolute(t *testing.T) {
	isolateTestProjectRoot(t)
	dataDir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dataDir)
	// 普通模式：protected_workspace.json 不存在 → ProtectedWorkspaceEnabled fail-open
	root := core.GetProjectRoot()
	// 清理注册表残留（测试隔离）
	approvedOutsideMu.Lock()
	approvedOutsidePath = map[string]time.Time{}
	approvedOutsideMu.Unlock()

	content := "# 自动提取记忆\n\n- **code_language** C\n"
	memFile := filepath.Join(dataDir, "memory", "facts.md")
	if err := os.MkdirAll(filepath.Dir(memFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// 工作目录内副本用于路径归一对照
	relDoc := filepath.Join(root, "doc.md")
	if err := os.WriteFile(relDoc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/agent/file", HandleAgentFile)

	// 1) 普通模式：工作目录外绝对路径直接 200（不再要求批准）
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/agent/file?path="+url.QueryEscape(memFile), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("普通模式绝对路径应 200，得到 %d: %s", w.Code, w.Body.String())
	}

	// 2) serve 元数据 + content 回读
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/agent/file?path="+url.QueryEscape(memFile), nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("绝对路径应 200，得到 %d: %s", w2.Code, w2.Body.String())
	}
	var meta struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Name != "facts.md" || meta.Kind != "text" || meta.Content != content {
		t.Errorf("serve 元数据不符: %+v", meta)
	}

	// 3) raw 下载
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/api/agent/file?path="+url.QueryEscape(memFile)+"&raw=1", nil))
	if w3.Code != http.StatusOK || w3.Body.String() != content {
		t.Errorf("raw 应 200 且内容一致，得到 %d", w3.Code)
	}

	// 4) 工作目录内相对路径：不受影响（回归）
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, httptest.NewRequest(http.MethodGet, "/api/agent/file?path=doc.md", nil))
	if w4.Code != http.StatusOK {
		t.Errorf("工作目录内相对路径应 200，得到 %d: %s", w4.Code, w4.Body.String())
	}

	// 5) 凭据文件（user_configs/ 下）任何模式都 400——明文 key 绝不走公开读口
	credDir := filepath.Join(dataDir, "user_configs")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cred := filepath.Join(credDir, "openid123.json")
	if err := os.WriteFile(cred, []byte(`{"api_key":"sk-secret"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, httptest.NewRequest(http.MethodGet, "/api/agent/file?path="+url.QueryEscape(cred), nil))
	if w5.Code == http.StatusOK {
		t.Error("user_configs 凭据文件不应经公开读口外发")
	}

	// 6) 受保护工作区模式：未批准的绝对路径回到 400，批准后放行
	if err := saveProtectedWorkspaceConfig(protectedWorkspaceConfig{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	w6 := httptest.NewRecorder()
	r.ServeHTTP(w6, httptest.NewRequest(http.MethodGet, "/api/agent/file?path="+url.QueryEscape(memFile), nil))
	if w6.Code != http.StatusBadRequest {
		t.Fatalf("受保护模式未批准应 400，得到 %d", w6.Code)
	}
	rememberApprovedOutsidePath(memFile)
	w7 := httptest.NewRecorder()
	r.ServeHTTP(w7, httptest.NewRequest(http.MethodGet, "/api/agent/file?path="+url.QueryEscape(memFile), nil))
	if w7.Code != http.StatusOK {
		t.Fatalf("受保护模式已批准应 200，得到 %d", w7.Code)
	}
	// 受保护模式下凭据文件即使被批准也拒（黑名单优先）
	rememberApprovedOutsidePath(cred)
	w8 := httptest.NewRecorder()
	r.ServeHTTP(w8, httptest.NewRequest(http.MethodGet, "/api/agent/file?path="+url.QueryEscape(cred), nil))
	if w8.Code == http.StatusOK {
		t.Error("凭据文件即使批准也不应 200")
	}
	// 恢复：保护开关配置文件删除，避免影响后续用例
	os.Remove(protectedWorkspaceConfigPath())
}
