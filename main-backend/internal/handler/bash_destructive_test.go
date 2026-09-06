package handler

import "testing"

// bash 是四件套收敛后的命令工具名（2026-08-29 起模型实际用它执行 shell）。
// YOLO 模式下破坏性命令保护是唯一防线——git checkout -- / git restore /
// rm -rf 走 bash 必须同样被拦，否则工作区可以被一条命令整体抹掉。
// 回归：2026-09-06 实锤（isDestructiveToolCall 只认 run_command，bash 漏网）。
func TestDestructiveToolCallCatchesBash(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    bool
	}{
		{"git checkout 恢复工作区", "git checkout -- internal/x.go", true},
		{"git checkout 点整个目录", "git checkout .", true},
		{"git restore 覆盖工作区", "git restore .", true},
		{"git reset hard", "git reset --hard", true},
		{"git clean", "git clean -fd", true},
		{"git rm", "git rm old.txt", true},
		{"rm 递归删除", "rm -rf node_modules", true},
		{"普通命令", "git status", false},
		{"安全读取", "ls -la", false},
	}
	for _, c := range cases {
		got := isDestructiveToolCall("bash", `{"command":"`+c.command+`"}`)
		if got != c.want {
			t.Errorf("[%s] bash %q 拦截=%v, want %v", c.name, c.command, got, c.want)
		}
	}
}