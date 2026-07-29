package handler

import (
	"path/filepath"
	"testing"
)

func TestDefaultMCPConfigLivesInUserData(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dataDir)
	t.Setenv("MCP_CONFIG", "")
	if got, want := mcpConfigPath(), filepath.Join(dataDir, "mcp.json"); got != want {
		t.Fatalf("默认 MCP 配置路径 = %q，期望 %q", got, want)
	}
}

func TestExplicitMCPConfigStillSupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-mcp.json")
	t.Setenv("MCP_CONFIG", path)
	if got := mcpConfigPath(); got != path {
		t.Fatalf("MCP_CONFIG 未生效: %q", got)
	}
}
