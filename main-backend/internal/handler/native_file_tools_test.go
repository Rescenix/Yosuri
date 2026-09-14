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

// TestNativeReadSecretFileMasked 验证「堵不如疏」（09-14 纠正）：密钥类文件
// （.env/.pem 等）不再整文件禁读，改为出口脱敏——能读到文件，但密钥值被
// maskSecretText 掩成 ***，模型拿不到明文。
func TestNativeReadSecretFileMasked(t *testing.T) {
	root := t.TempDir()
	secretFiles := []string{
		".env", ".env.local", ".env.production",
		"server.pem", "id_rsa", "id_ed25519",
		"credentials.json", "service-account.json",
	}
	// 值长度 >= 8 才会触发掩码（maskMinValueLen）
	content := "SECRET_KEY=abcdefgh123456\n"
	for _, name := range secretFiles {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		res, err := callNativeFileTool("read_file", nativeArgs(t, map[string]any{"path": path}))
		if err != nil {
			t.Errorf("密钥文件 %s 应可读（脱敏而非拦截），实得拦截: %v", name, err)
			continue
		}
		if strings.Contains(res.Text, "abcdefgh123456") {
			t.Errorf("密钥文件 %s 泄漏了明文密钥值: %q", name, res.Text)
		}
		if !strings.Contains(res.Text, "SECRET_KEY=***") {
			t.Errorf("密钥文件 %s 应显示掩码 SECRET_KEY=***，实得: %q", name, res.Text)
		}
	}
}

// TestNativeReadNonSecretDocsAllowed 回归（2026-09-14 实锤）：门面文档/依赖清单/
// 协作规范是**写保护**名单（readme/license/package.json/agents.md），不是禁读名单。
// 只读它们没有泄密风险，必须能正常读取；密钥值靠出口脱敏兜底，不做整文件拦截。
func TestNativeReadNonSecretDocsAllowed(t *testing.T) {
	root := t.TempDir()
	docs := []string{
		"README.md", "README.zh-CN.md", "LICENSE", "license.txt",
		"package.json", "go.mod", "AGENTS.md", ".cursorrules", ".gitignore",
	}
	for _, name := range docs {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("# doc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range docs {
		path := filepath.Join(root, name)
		res, err := callNativeFileTool("read_file", nativeArgs(t, map[string]any{"path": path}))
		if err != nil {
			t.Errorf("门面文档 %s 应可读，实得拦截: %v", name, err)
		} else if !strings.Contains(res.Text, "# doc") {
			t.Errorf("门面文档 %s 读到了但内容不对: %q", name, res.Text)
		}
	}
}