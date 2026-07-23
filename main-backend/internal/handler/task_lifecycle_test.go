package handler

import (
	"strings"
	"testing"
)

// 历史落盘只发生在成功收尾，所以空 Status 也必须按"已完成"读，
// 否则存量老会话全会被当成待办任务，正是要修的那个 bug。
func TestTaskDone(t *testing.T) {
	cases := []struct {
		name string
		msg  DSMessage
		want bool
	}{
		{"显式已完成", DSMessage{Role: "user", Status: taskStatusCompleted}, true},
		{"存量老数据(空状态)", DSMessage{Role: "user", Status: ""}, true},
		{"assistant 不是任务", DSMessage{Role: "assistant", Status: taskStatusCompleted}, false},
		{"tool 不是任务", DSMessage{Role: "tool"}, false},
	}
	for _, c := range cases {
		if got := taskDone(c.msg); got != c.want {
			t.Errorf("%s: taskDone=%v，期望 %v", c.name, got, c.want)
		}
	}
}

// 核心行为：历史任务被打标，当前任务保持原样。
func TestBuildChatMessagesMarksCompletedTasks(t *testing.T) {
	history := []DSMessage{
		{Role: "user", Content: "把 fibonacci.py 改成输出我的世界", Status: taskStatusCompleted},
		{Role: "assistant", Content: "改好了"},
	}
	msgs := buildChatMessages("SYS", history, "新任务：写一个登录页")

	if len(msgs) != 4 {
		t.Fatalf("消息数应为 4（system+2历史+当前），实得 %d", len(msgs))
	}
	if !strings.HasPrefix(msgs[1]["content"], completedTaskPrefix) {
		t.Errorf("历史任务没被打标: %q", msgs[1]["content"])
	}
	if !strings.Contains(msgs[1]["content"], "fibonacci") {
		t.Error("打标后原文丢失")
	}
	if strings.HasPrefix(msgs[2]["content"], completedTaskPrefix) {
		t.Error("assistant 回复不该被打标")
	}
	// 最关键的一条：当前任务必须是干净的，它是唯一要执行的
	if msgs[3]["content"] != "新任务：写一个登录页" {
		t.Errorf("当前任务被改动了: %q", msgs[3]["content"])
	}
}

// 回归：上下文压缩用 lastIndexOfTask 按内容精确匹配定位当前任务，
// 给历史加前缀绝不能破坏它——匹配不上会导致 preamble 边界算错，
// 进而把当前任务本身折叠进摘要里。
func TestTaskPrefixDoesNotBreakCompactionAnchor(t *testing.T) {
	task := "写一个登录页"
	history := []DSMessage{
		{Role: "user", Content: task, Status: taskStatusCompleted}, // 同名的历史任务
		{Role: "assistant", Content: "上次做过了"},
	}
	built := buildChatMessages("SYS", history, task)

	msgs := make([]map[string]any, len(built))
	for i, m := range built {
		msgs[i] = map[string]any{"role": m["role"], "content": m["content"]}
	}

	idx := lastIndexOfTask(msgs, task)
	if idx != len(msgs)-1 {
		t.Fatalf("应锚定到最后那条未打标的当前任务(idx=%d)，实得 %d", len(msgs)-1, idx)
	}
	// 打标过的同名历史任务不该被误认成当前任务
	if idx == 1 {
		t.Error("锚到了历史任务上——前缀没起到区分作用")
	}
}

// Status 必须能穿过持久化往返，否则重启后全变空、标记形同虚设。
func TestStatusSurvivesPersistRoundTrip(t *testing.T) {
	orig := []DSMessage{
		{Role: "user", Content: "任务A", Status: taskStatusCompleted},
		{Role: "assistant", Content: "好了"},
	}
	back := fromPersistedMessages(toPersistedMessages(orig))
	if back[0].Status != taskStatusCompleted {
		t.Errorf("user 的 Status 丢失: %q", back[0].Status)
	}
	if back[1].Status != "" {
		t.Errorf("assistant 不该凭空多出状态: %q", back[1].Status)
	}
}
