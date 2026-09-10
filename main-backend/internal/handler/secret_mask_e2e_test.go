package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/ai/core"
)

// TestNativeReadFile_MasksSecrets 端到端：read 工具出口必须看不到明文 key。
func TestNativeReadFile_MasksSecrets(t *testing.T) {
	isolateTestProjectRoot(t)
	root := core.GetProjectRoot()
	p := filepath.Join(root, "settings.json")
	secret := "sk-proj-REALSECRETVALUE123456"
	if err := os.WriteFile(p, []byte(`{"model":"gpt","api_key":"`+secret+`","name":"yosuri"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := nativeReadFile(map[string]any{"path": "settings.json"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Text, secret) {
		t.Errorf("read 出口泄漏明文 key: %s", res.Text)
	}
	if !strings.Contains(res.Text, `"api_key":"***"`) {
		t.Errorf("应看到掩码形态: %s", res.Text)
	}
	if !strings.Contains(res.Text, `"name":"yosuri"`) {
		t.Errorf("非敏感字段不应被误伤: %s", res.Text)
	}
}
