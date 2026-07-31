package memorydir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 用临时 HOME 隔离测试，不碰真实 ~/rescene_data
func withTempHome(t *testing.T, fn func()) {
	t.Helper()
	old, _ := os.UserHomeDir()
	tmp := t.TempDir()
	// path() 内部用 os.UserHomeDir()，通过 HOME 环境变量重定向（Windows 认 USERPROFILE）
	os.Setenv("USERPROFILE", tmp)
	os.Setenv("HOME", tmp)
	defer func() {
		os.Setenv("USERPROFILE", old)
		os.Setenv("HOME", old)
	}()
	fn()
}

func TestPinAndReadPinned(t *testing.T) {
	withTempHome(t, func() {
		if err := Pin("P01", "用户喜欢简短回复"); err != nil {
			t.Fatal(err)
		}
		if err := Pin("P02", "项目用 Go + Vue"); err != nil {
			t.Fatal(err)
		}
		// 同 pid 覆盖
		if err := Pin("P01", "用户喜欢非常简短回复"); err != nil {
			t.Fatal(err)
		}
		got := ReadPinned()
		if !strings.Contains(got, "P01") || !strings.Contains(got, "P02") {
			t.Fatalf("pinned 内容不完整: %s", got)
		}
		if strings.Contains(got, "喜欢简短回复") && strings.Contains(got, "喜欢非常简短回复") {
			t.Fatalf("P01 未覆盖: %s", got)
		}
		if !strings.Contains(got, "非常简短") {
			t.Fatalf("P01 应被新内容覆盖: %s", got)
		}
		// 落盘验证
		if _, err := os.Stat(filepath.Join(path(), "pinned.md")); err != nil {
			t.Fatalf("pinned.md 未落盘: %v", err)
		}
	})
}

func TestHandoffWriteAndRead(t *testing.T) {
	withTempHome(t, func() {
		if err := HandoffWrite("正在重构记忆系统"); err != nil {
			t.Fatal(err)
		}
		got := ReadHandoff()
		if got != "正在重构记忆系统" {
			t.Fatalf("handoff 读写不一致: %q", got)
		}
		// 覆盖式
		if err := HandoffWrite("新的工作态"); err != nil {
			t.Fatal(err)
		}
		if ReadHandoff() != "新的工作态" {
			t.Fatalf("handoff 应覆盖而非追加: %q", ReadHandoff())
		}
	})
}

func TestRememberAndSearch(t *testing.T) {
	withTempHome(t, func() {
		if err := Remember("preferences", "用户偏好", "用户喜欢中文回复"); err != nil {
			t.Fatal(err)
		}
		got := Search("偏好")
		if got == "" {
			t.Fatal("Search 应命中 preferences")
		}
		if !strings.Contains(got, "中文回复") {
			t.Fatalf("Search 应返回文件内容: %s", got)
		}
		if Search("不存在的关键词xyz") != "" {
			t.Fatal("无命中时应返回空串")
		}
		// index.md 更新验证
		idx := ReadIndex()
		if !strings.Contains(idx, "[[preferences]]") {
			t.Fatalf("index 未登记 preferences: %s", idx)
		}
	})
}
