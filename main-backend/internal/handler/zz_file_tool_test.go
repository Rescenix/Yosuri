package handler

// 「文件」工具后端的测试：读写越界拦截、写端点往返、树构建跳过垃圾目录。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempRepoRoot 把 GitRepoRoot 换成临时目录，测试结束自动还原，
// 不碰真实仓库根（也不影响别的测试用到的 GitRepoRoot）。
func withTempRepoRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := GitRepoRoot
	GitRepoRoot = dir
	t.Cleanup(func() { GitRepoRoot = old })
	return dir
}

func TestResolveRepoPath_BlocksSiblingPrefixAttack(t *testing.T) {
	root := withTempRepoRoot(t)
	// 制造一个"同前缀但其实是兄弟目录"的场景：root 是 .../xxx，
	// 攻击路径拼出 .../xxx-evil/secret.txt —— 老版 strings.HasPrefix(fullPath, root)
	// 会被这个骗过去（字符串前缀匹配，没有路径分隔符边界）。
	evilSibling := root + "-evil"
	if err := os.MkdirAll(evilSibling, 0o755); err != nil {
		t.Fatalf("准备测试目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evilSibling, "secret.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("准备测试文件失败: %v", err)
	}

	// 用 ../ 从 root 拼到兄弟目录
	rel := "../" + filepath.Base(evilSibling) + "/secret.txt"
	if _, ok := resolveRepoPath(rel); ok {
		t.Fatalf("resolveRepoPath(%q) 应该拒绝越界到兄弟目录，却放行了", rel)
	}
}

func TestResolveRepoPath_AllowsRootItself(t *testing.T) {
	withTempRepoRoot(t)
	if _, ok := resolveRepoPath("."); !ok {
		t.Fatalf("resolveRepoPath(\".\") 应该允许（就是仓库根本身）")
	}
	if _, ok := resolveRepoPath("sub/dir/file.txt"); !ok {
		t.Fatalf("resolveRepoPath 应该允许仓库根内的相对路径")
	}
}

func TestFileWriteHandler_RoundTrip(t *testing.T) {
	withTempRepoRoot(t)

	body, _ := json.Marshal(fileWriteRequest{Path: "notes/a.md", Content: "# hello\n改过了"})
	req := httptest.NewRequest(http.MethodPost, "/api/file", bytes.NewReader(body))
	w := httptest.NewRecorder()
	FileWriteHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("写入应返回 200，got %d: %s", w.Code, w.Body.String())
	}

	// 用真实 FileReadHandler 读回来，确认端到端一致（不是绕过 handler 直接查磁盘）
	readReq := httptest.NewRequest(http.MethodGet, "/api/file?path=notes/a.md", nil)
	readW := httptest.NewRecorder()
	FileReadHandler(readW, readReq)
	if readW.Code != http.StatusOK {
		t.Fatalf("读回应返回 200，got %d: %s", readW.Code, readW.Body.String())
	}
	if got := readW.Body.String(); got != "# hello\n改过了" {
		t.Fatalf("读回内容不一致: got %q", got)
	}
}

func TestFileWriteHandler_RejectsOutsideRoot(t *testing.T) {
	withTempRepoRoot(t)
	body, _ := json.Marshal(fileWriteRequest{Path: "../outside.txt", Content: "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/file", bytes.NewReader(body))
	w := httptest.NewRecorder()
	FileWriteHandler(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("越界写入应返回 403，got %d", w.Code)
	}
}

// 防止「文件」工具点开一个几百 MB/几 GB 的二进制（仓库根/models 下都有）把浏览器卡死。
func TestFileReadHandler_RejectsOversizedFile(t *testing.T) {
	root := withTempRepoRoot(t)
	big := make([]byte, maxReadableFileBytes+1024)
	if err := os.WriteFile(filepath.Join(root, "huge.bin"), big, 0o644); err != nil {
		t.Fatalf("准备大文件失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/file?path=huge.bin", nil)
	w := httptest.NewRecorder()
	FileReadHandler(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("超限文件应返回 413，got %d: %s", w.Code, w.Body.String())
	}
}

func TestFileReadHandler_RejectsDirectory(t *testing.T) {
	root := withTempRepoRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/file?path=adir", nil)
	w := httptest.NewRecorder()
	FileReadHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("读目录应返回 400，got %d: %s", w.Code, w.Body.String())
	}
}

func TestFileWriteHandler_RejectsGetMethod(t *testing.T) {
	withTempRepoRoot(t)
	req := httptest.NewRequest(http.MethodGet, "/api/file", nil)
	w := httptest.NewRecorder()
	FileWriteHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET 打写入端点应返回 405，got %d", w.Code)
	}
}

func TestWatchedVersionDetectsWriteAndDelete(t *testing.T) {
	root := withTempRepoRoot(t)
	path := filepath.Join(root, "watched.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("准备测试文件失败: %v", err)
	}
	before := watchedVersion(path)
	if err := os.WriteFile(path, []byte("new content"), 0o644); err != nil {
		t.Fatalf("改写测试文件失败: %v", err)
	}
	afterWrite := watchedVersion(path)
	if !afterWrite.changedFrom(before) {
		t.Fatal("改写后的文件版本应该变化")
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("删除测试文件失败: %v", err)
	}
	if !watchedVersion(path).changedFrom(afterWrite) {
		t.Fatal("删除后的文件版本应该变化")
	}
}

// 树构建要跳过 .gocache/.mimocode 这类几千文件的垃圾目录，否则「文件」工具的树
// 会被这些目录塞爆——2026-07-24 在真实仓库上实测过（.gocache 3800+、.mimocode 3400+ 文件）。
func TestBuildFileTree_SkipsJunkDirs(t *testing.T) {
	root := withTempRepoRoot(t)
	for _, junk := range []string{".gocache", ".mimocode", "node_modules", ".git", "__pycache__", ".pytest_cache"} {
		full := filepath.Join(root, junk)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("建垃圾目录失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(full, "junk.file"), []byte("x"), 0o644); err != nil {
			t.Fatalf("建垃圾文件失败: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "real.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("建正常文件失败: %v", err)
	}

	nodes, err := buildFileTree(root, root)
	if err != nil {
		t.Fatalf("buildFileTree 失败: %v", err)
	}
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	joined := strings.Join(names, ",")
	for _, junk := range []string{".gocache", ".mimocode", "node_modules", ".git", "__pycache__", ".pytest_cache"} {
		if strings.Contains(joined, junk) {
			t.Errorf("垃圾目录 %s 不该出现在树里，实际节点: %v", junk, names)
		}
	}
	if !strings.Contains(joined, "real.go") {
		t.Errorf("正常文件 real.go 应该出现在树里，实际节点: %v", names)
	}
}
