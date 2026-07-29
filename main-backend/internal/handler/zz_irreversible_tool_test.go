package handler

import "testing"

// TestIsIrreversibleTool 验证 YOLO 模式下「不可逆文件操作强制拦截」的判定边界：
// 删除/移动/重命名必须返回 true，其余写操作（write/edit/create）返回 false。
func TestIsIrreversibleTool(t *testing.T) {
	// 必须拦截的不可逆操作
	mustBlock := []string{
		"delete_file",
		"delete_directory",
		"move_file",
		"mcp__fs__delete_file",
		"mcp__fs__delete_directory",
		"mcp__fs__move_file",
		// 前缀兜底也应命中
		"mcp__fs__delete_X",
		"mcp__fs__move_X",
		"mcp__fs__rename_X",
	}
	for _, name := range mustBlock {
		if !isIrreversibleTool(name) {
			t.Errorf("%s 是不可逆操作，YOLO 下应强制拦截（isIrreversibleTool 返回 true），实际返回 false", name)
		}
	}

	// YOLO 下应畅通的写操作（不拦截）
	shouldPass := []string{
		"mcp__fs__write_file",
		"mcp__fs__edit_file",
		"mcp__fs__create_file",
		"mcp__fs__create_directory",
		// 只读类更不应命中
		"mcp__fs__read_file",
		"mcp__fs__list_directory",
		"mcp__fs__search_files",
		"mcp__shell__run", // 非 fs 前缀，不归这里管
	}
	for _, name := range shouldPass {
		if isIrreversibleTool(name) {
			t.Errorf("%s 是可逆/非 fs 操作，不该被 isIrreversibleTool 判定为不可逆，实际返回 true", name)
		}
	}
}
