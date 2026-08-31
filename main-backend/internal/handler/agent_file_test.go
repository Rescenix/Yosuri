package handler

// agent_file_test.go —— 文件交付链路验证：
//   1. deliverableFromPath 只认可交付扩展名，源码/配置不弹卡片；
//   2. HandleAgentFile 端点按扩展名分类 + 文本回读 + raw 下载。
// 不依赖运行中的 8080 服务，纯内存 + 临时目录，可重复。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
