package handler

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 只有"会改文件内容的 fs 工具" + "前端后缀"两个条件同时成立才该弹预览。
func TestIsFrontendEdit(t *testing.T) {
	yes := []struct{ tool, args string }{
		{"mcp__fs__write_file", `{"path":"src/components/Card.vue"}`},
		{"mcp__fs__edit_file", `{"path":"C:\\Pro2026\\re0\\main-frontend\\a.css"}`},
		{"mcp__fs__create_file", `{"path":"pages/index.html"}`},
		{"mcp__fs__write_file", `{"path":"App.JSX"}`}, // 后缀大小写不敏感
	}
	for _, c := range yes {
		if !isFrontendEdit(c.tool, c.args) {
			t.Errorf("应判为前端改动: %s %s", c.tool, c.args)
		}
	}

	no := []struct {
		tool, args, why string
	}{
		{"mcp__fs__write_file", `{"path":"main.go"}`, "后端文件"},
		{"mcp__fs__write_file", `{"path":"README.md"}`, "文档"},
		{"mcp__fs__read_text_file", `{"path":"a.vue"}`, "只读工具不该弹预览"},
		{"mcp__fs__list_directory", `{"path":"src"}`, "列目录"},
		{"mcp__shell__run", `{"command":"npm run build"}`, "非文件工具"},
		{"mcp__fs__write_file", `{`, "坏 JSON 必须安静返回 false"},
		{"mcp__fs__write_file", `{"path":""}`, "空路径"},
		{"mcp__fs__write_file", `{"content":"x"}`, "没有 path 字段"},
	}
	for _, c := range no {
		if isFrontendEdit(c.tool, c.args) {
			t.Errorf("不该判为前端改动(%s): %s %s", c.why, c.tool, c.args)
		}
	}
}

// .vuex / .jsonc 这类"以前端后缀开头但其实不是"的名字不能误判——
// 用的是 HasSuffix，这里做个防回归。
func TestIsFrontendEditSuffixNotPrefix(t *testing.T) {
	if isFrontendEdit("mcp__fs__write_file", `{"path":"store.vuex"}`) {
		t.Error(".vuex 不是前端渲染文件，不该命中")
	}
}

// aliveFrontendURL 得真的探到活着的前端端口。
// 4322 可能已经被真实的 dev server 占着，也可能空闲——两种情况都要能验，
// 所以先尝试自己占，占不上就说明本来就有服务在跑，同样满足"该端口存活"的前提。
func TestAliveFrontendURL(t *testing.T) {
	const port = 4322 // 候选清单里的本项目前端端口
	want := fmt.Sprintf("http://localhost:%d", port)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		// 端口本来空闲：自己起一个只接受再关闭的假服务，用完释放
		defer ln.Close()
		go func() {
			for {
				conn, aerr := ln.Accept()
				if aerr != nil {
					return
				}
				conn.Close()
			}
		}()
	} // err != nil 说明已有真实服务在监听，直接往下测

	if got := aliveFrontendURL(); got != want {
		t.Fatalf("%d 上有服务在监听时应探到 %s，实得 %q", port, want, got)
	}
}

// 候选清单里的前端项必须排在后端项前面，否则 aliveFrontendURL 的"取第一个"会拿错东西。
func TestFrontendCandidatesComeFirst(t *testing.T) {
	seenBackend := false
	for _, c := range previewCandidates {
		if c.Category != "frontend" {
			seenBackend = true
			continue
		}
		if seenBackend {
			t.Errorf("前端候选 %s(%d) 排在了非前端项后面，会影响自动预览选址", c.Name, c.Port)
		}
	}
}

func TestValidatePreviewTargetWS(t *testing.T) {
	previewBrowserMu.Lock()
	previous := previewBrowser
	previewBrowser = &managedPreviewBrowser{port: "43117"}
	previewBrowserMu.Unlock()
	t.Cleanup(func() {
		previewBrowserMu.Lock()
		previewBrowser = previous
		previewBrowserMu.Unlock()
	})

	valid := []string{
		"ws://127.0.0.1:9222/devtools/page/abc",
		"ws://localhost:9222/devtools/page/abc",
		"ws://[::1]:9222/devtools/page/abc",
		"ws://127.0.0.1:43117/devtools/page/managed",
	}
	for _, raw := range valid {
		if _, err := validatePreviewTargetWS(raw); err != nil {
			t.Errorf("合法的本机 CDP target 被拒绝 %q: %v", raw, err)
		}
	}

	invalid := []string{
		"ws://127.0.0.1:9223/devtools/page/abc",
		"ws://127.0.0.1:43118/devtools/page/unmanaged",
		"ws://192.168.1.2:9222/devtools/page/abc",
		"ws://127.0.0.1:9222/devtools/browser/abc",
		"wss://127.0.0.1:9222/devtools/page/abc",
		"http://127.0.0.1:9222/devtools/page/abc",
	}
	for _, raw := range invalid {
		if _, err := validatePreviewTargetWS(raw); err == nil {
			t.Errorf("危险或无效的 CDP target 未被拒绝: %q", raw)
		}
	}
}

func TestBundledBrowserCandidatesAreRelativeToAppExecutable(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "ResceneAgent.exe")
	candidates := bundledBrowserCandidates(appPath)
	if len(candidates) == 0 {
		t.Fatal("随包 Chromium 候选路径不能为空")
	}
	wantRoot := filepath.Join(filepath.Dir(appPath), "runtime", "chromium") + string(os.PathSeparator)
	if !strings.HasPrefix(candidates[0], wantRoot) {
		t.Fatalf("首选随包 Chromium 应位于 %s，实得 %s", wantRoot, candidates[0])
	}
}

func TestReadDevToolsActivePort(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte("43117\n/devtools/browser/test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	port, err := readDevToolsActivePort(dir)
	if err != nil {
		t.Fatalf("读取动态 CDP 端口失败: %v", err)
	}
	if port != "43117" {
		t.Fatalf("动态 CDP 端口不符: %s", port)
	}
}

func TestValidatePreviewTargetURL(t *testing.T) {
	if _, err := validatePreviewTargetURL("file:///C:/workspace/index.html"); err != nil {
		t.Fatalf("合法 file URL 被拒绝: %v", err)
	}
	for _, raw := range []string{
		"http://127.0.0.1:4322",
		"https://example.com",
		"file://",
		"ws://127.0.0.1:9222/devtools/page/abc",
	} {
		if _, err := validatePreviewTargetURL(raw); err == nil {
			t.Errorf("不安全或无效的预览 URL 未被拒绝: %q", raw)
		}
	}
}
