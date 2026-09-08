package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nativeArgs(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNativeFileToolsReadEditGrepGlob(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src", "hello.go")

	if _, err := callNativeFileTool("write_file", nativeArgs(t, map[string]any{
		"path": path, "content": "package demo\n\nfunc Hello() string {\n\treturn \"hello\"\n}\n",
	})); err != nil {
		t.Fatalf("write_file: %v", err)
	}

	read, err := callNativeFileTool("read_file", nativeArgs(t, map[string]any{
		"path": path, "offset": 3, "limit": 2,
	}))
	if err != nil || !strings.Contains(read.Text, "3:func Hello") || !strings.Contains(read.Text, "4:\treturn") {
		t.Fatalf("read_file 结果不对: err=%v text=%q", err, read.Text)
	}

	edit, err := callNativeFileTool("edit_file", nativeArgs(t, map[string]any{
		"path": path, "old_string": " func Hello() string {\n return \"hello\"\n }",
		"new_string": "func Hello() string {\n\treturn \"hi\"\n}",
	}))
	if err != nil || !strings.Contains(edit.Text, "空白") {
		t.Fatalf("edit_file 空白容错失败: err=%v text=%q", err, edit.Text)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `return "hi"`) {
		t.Fatalf("edit_file 未写入目标内容: %s", data)
	}

	grep, err := callNativeFileTool("grep", nativeArgs(t, map[string]any{
		"path": root, "pattern": `return "hi"`, "type": "go",
	}))
	if err != nil || !strings.Contains(grep.Text, "hello.go:4:") {
		t.Fatalf("grep 结果不对: err=%v text=%q", err, grep.Text)
	}

	glob, err := callNativeFileTool("glob", nativeArgs(t, map[string]any{
		"path": root, "pattern": "**/*.go",
	}))
	if err != nil || !strings.Contains(glob.Text, "hello.go") {
		t.Fatalf("glob 结果不对: err=%v text=%q", err, glob.Text)
	}
}

func TestNativeEditRejectsAmbiguousMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dup.txt")
	if err := os.WriteFile(path, []byte("same\nsame\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := callNativeFileTool("edit_file", nativeArgs(t, map[string]any{
		"path": path, "old_string": "same", "new_string": "changed",
	}))
	if err == nil || !strings.Contains(err.Error(), "出现 2 次") {
		t.Fatalf("应拒绝歧义替换，实得 %v", err)
	}
}

// TestNativeReadSensitiveFileBlocked 验证敏感文件读取拦截：.env/.pem/.ssh 等
// 密钥文件被 read 时必须返回安全闸门错误，不能返回文件内容。
func TestNativeReadSensitiveFileBlocked(t *testing.T) {
	root := t.TempDir()
	sensitiveFiles := []string{
		".env", ".env.local", ".env.production",
		"server.pem", "id_rsa", "id_ed25519",
		"credentials.json", "service-account.json",
	}
	for _, name := range sensitiveFiles {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("SECRET_KEY=abc123\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := callNativeFileTool("read_file", nativeArgs(t, map[string]any{"path": path}))
		if err == nil {
			t.Errorf("敏感文件 %s 被 read 拦截失败——返回了内容", name)
			continue
		}
		if !strings.Contains(err.Error(), "安全闸门") {
			t.Errorf("敏感文件 %s 被拦但错误信息不含「安全闸门」: %v", name, err)
		}
	}
	// 反测：普通文件必须能正常读到
	normalPath := filepath.Join(root, "normal.txt")
	if err := os.WriteFile(normalPath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := callNativeFileTool("read_file", nativeArgs(t, map[string]any{"path": normalPath}))
	if err != nil || !strings.Contains(res.Text, "hello world") {
		t.Errorf("普通文件 normal.txt 读取应成功，实得 err=%v text=%q", err, res.Text)
	}
}
