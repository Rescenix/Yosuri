package core

import (
	"os"
	"path/filepath"
	"testing"
)

// withIsolatedWorkdirState 把状态文件重定向到临时目录，防止测试写坏
// 真实用户的 ~/rescene_data/workdir.txt（之前吃过这个亏，SetProjectRoot 落盘
// 一旦跑进临时目录，就会把正在跑的 agent 工作目录悄悄改没了）
func withIsolatedWorkdirState(t *testing.T) {
	t.Helper()
	t.Setenv("SHANXI_WORKDIR_STATE_FILE", filepath.Join(t.TempDir(), "workdir.txt"))
	original := GetProjectRoot()
	t.Cleanup(func() {
		projectRootAtomic.Store(original)
	})
}

func TestSetProjectRootPersistsAndTakesEffect(t *testing.T) {
	withIsolatedWorkdirState(t)

	// SetProjectRoot 拒绝不存在的目录，避免切到一个坏路径把后续所有工具调用带崩
	if err := SetProjectRoot(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatalf("expected error for nonexistent dir")
	}

	dir := t.TempDir()
	if err := SetProjectRoot(dir); err != nil {
		t.Fatalf("SetProjectRoot failed: %v", err)
	}
	if got := GetProjectRoot(); got != dir {
		t.Fatalf("GetProjectRoot() = %q, want %q", got, dir)
	}

	// 落盘持久化：状态文件里应该是刚设的这个路径
	data, err := os.ReadFile(workdirStateFile())
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	if string(data) != dir {
		t.Fatalf("persisted workdir = %q, want %q", string(data), dir)
	}

	// 工具调用实际用的就是这个值——file_handler 风格的路径拼接验证
	resolved := filepath.Join(GetProjectRoot(), "sub", "file.txt")
	if filepath.Dir(filepath.Dir(resolved)) != dir {
		t.Fatalf("path resolution didn't use updated root: %s", resolved)
	}
}

func TestSetProjectRootRejectsFile(t *testing.T) {
	withIsolatedWorkdirState(t)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := SetProjectRoot(filePath); err == nil {
		t.Fatalf("expected error when target is a file, not a directory")
	}
}

func TestLoadInitialProjectRootRespectsPersistedState(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(t.TempDir(), "workdir.txt")
	if err := os.WriteFile(stateFile, []byte(dir), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	t.Setenv("SHANXI_WORKDIR_STATE_FILE", stateFile)

	if got := loadInitialProjectRoot(); got != dir {
		t.Fatalf("loadInitialProjectRoot() = %q, want %q", got, dir)
	}
}
