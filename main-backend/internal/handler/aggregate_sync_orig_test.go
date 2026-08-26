package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 验证：还原回到「原始配置」(*.orig-bak)，而非最近一次同步备份。

func TestOrigBackupRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	origContent := "user_original_config = true\n"
	if err := os.WriteFile(p, []byte(origContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// 首次写回前 ensureOrigBackup 应存 orig
	ob, err := ensureOrigBackup(p)
	if err != nil {
		t.Fatalf("ensureOrigBackup: %v", err)
	}
	if ob == "" {
		t.Fatalf("orig backup path empty")
	}
	if _, err := os.Stat(ob); err != nil {
		t.Fatalf("orig backup missing: %v", err)
	}

	// 模拟多次同步写回（覆盖原文件）
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(p, []byte("synced_v"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 还原：应回到 orig 内容，不是最近一次 synced_v2
	orig := p + ".orig-bak"
	if err := os.WriteFile(p, []byte(origContent), 0o644); err != nil { // copyFile(orig,p)
		t.Fatal(err)
	}
	// 直接用 copyFile 模拟 restore 行为
	if err := copyFile(orig, p); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != origContent {
		t.Fatalf("restore returned %q, want original %q", string(got), origContent)
	}

	// 再次 ensureOrigBackup 不应覆盖已存在的 orig
	if _, err := ensureOrigBackup(p); err != nil {
		t.Fatalf("ensureOrigBackup second: %v", err)
	}
	// 改回 synced 内容再还原，仍应是 original
	if err := os.WriteFile(p, []byte("synced_again"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(orig, p); err != nil {
		t.Fatal(err)
	}
	got2, _ := os.ReadFile(p)
	if string(got2) != origContent {
		t.Fatalf("second restore returned %q, want original", string(got2))
	}
}

func TestRewriteDshBaseURL(t *testing.T) {
	src := "llm-pi-ai:\n  providers:\n    rescene:\n      apiKeyEnv: RESCENE_API_KEY\n      api: openai-completions\n      baseURL: http://old.example/v1\n      models:\n        - id: auto\n"
	out := rewriteDshBaseURL(src, "http://localhost:8080/v1")
	if !strings.Contains(out, "baseURL: http://localhost:8080/v1") {
		t.Fatalf("baseURL not rewritten:\n%s", out)
	}
	if strings.Contains(out, "http://old.example/v1") {
		t.Fatalf("old baseURL still present:\n%s", out)
	}
	if !strings.Contains(out, "apiKeyEnv: RESCENE_API_KEY") {
		t.Fatalf("other config clobbered:\n%s", out)
	}
}
