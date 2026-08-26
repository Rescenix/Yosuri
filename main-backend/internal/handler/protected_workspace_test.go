package handler

import (
	"path/filepath"
	"testing"

	"backend/internal/ai/core"
)

func TestProtectedWorkspaceLimitsFilesystemMCPToProjectRoot(t *testing.T) {
	t.Setenv("RESCENE_DATA_DIR", t.TempDir())
	if ProtectedWorkspaceEnabled() {
		t.Fatal("新的受保护工作区配置应默认关闭")
	}
	if err := saveProtectedWorkspaceConfig(protectedWorkspaceConfig{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if !ProtectedWorkspaceEnabled() {
		t.Fatal("保存开启状态后应读取为开启")
	}
	want := filepath.Clean(core.GetProjectRoot())
	if got := fsAllowedDirs(); len(got) != 1 || got[0] != want {
		t.Fatalf("受保护模式的 filesystem 范围 = %v, want [%s]", got, want)
	}
}
