package handler

import (
	"strings"
	"testing"
)

func TestWorkflowHistoryContentKeepsPartialSteps(t *testing.T) {
	got := workflowHistoryContent(
		taskStatusFailed,
		"模型源返回 500",
		[]string{
			`read_file({"path":"a.go"}) => package main`,
			`apply_patch(...) => 已更新 a.go`,
		},
		nil,
	)
	if !strings.Contains(got, "模型源返回 500") ||
		!strings.Contains(got, "apply_patch") ||
		!strings.Contains(got, "中断前已执行步骤摘要") {
		t.Fatalf("失败历史没有保留原因和已执行步骤: %q", got)
	}

	completed := workflowHistoryContent(
		taskStatusCompleted,
		"任务完成",
		[]string{"不应重复塞进成功答复"},
		nil,
	)
	if strings.Contains(completed, "已执行步骤摘要") {
		t.Fatalf("成功历史只应保存最终答复: %q", completed)
	}
}

func TestPersistWorkflowHistoryUpsertsResumeResult(t *testing.T) {
	withIsolatedSessionDir(t)
	store := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(store) })
	runner := NewWorkflowRunner(NewChatHandler(store))

	runner.persistWorkflowHistory(
		"session_1", "workflow_1", "修复登录页", taskStatusFailed,
		"第一次执行失败", "deepseek-v4-flash", []string{"apply_patch => 已更新 app.ts"}, []FlowBlock{
			{Type: "tool", Name: "apply_patch", Status: "ok"},
		},
	)
	failed := store.Get("session_1")
	if len(failed) != 2 || failed[0].Status != taskStatusFailed {
		t.Fatalf("首次失败应写入一组历史: %+v", failed)
	}
	if !strings.Contains(failed[1].Content, "apply_patch") {
		t.Fatalf("失败历史没有部分步骤: %q", failed[1].Content)
	}

	runner.persistWorkflowHistory(
		"session_1", "workflow_1", "修复登录页", taskStatusCompleted,
		"续跑后完成", "", nil, []FlowBlock{{Type: "intent", Text: "续跑后完成"}},
	)
	completed := store.Get("session_1")
	if len(completed) != 2 {
		t.Fatalf("同一 workflow 续跑不应追加重复消息，实得 %d 条", len(completed))
	}
	if completed[0].Status != taskStatusCompleted || completed[1].Content != "续跑后完成" {
		t.Fatalf("续跑结果没有原位更新: %+v", completed)
	}
	if completed[0].WorkflowID != "workflow_1" || completed[1].WorkflowID != "workflow_1" {
		t.Fatalf("workflow_id 未绑定到消息对: %+v", completed)
	}
	reloaded := NewSessionStore(ChatSessionsDomain)
	disk := reloaded.Get("session_1")
	if len(disk) != 2 || disk[0].Status != taskStatusCompleted ||
		disk[0].WorkflowID != "workflow_1" || disk[1].Content != "续跑后完成" {
		t.Fatalf("终态没有同步持久化或重启后丢字段: %+v", disk)
	}
}

func TestUpsertWorkflowPairPreservesOtherMessages(t *testing.T) {
	withIsolatedSessionDir(t)
	store := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(store) })

	store.Append("s", DSMessage{Role: "user", Content: "更早任务"})
	store.Append("s", DSMessage{Role: "assistant", Content: "更早回复"})
	store.UpsertWorkflowPair(
		"s", "wf",
		DSMessage{Role: "user", Content: "当前任务", Status: taskStatusInterrupted},
		DSMessage{Role: "assistant", Content: "停止了"},
	)
	store.Append("s", DSMessage{Role: "user", Content: "更晚任务"})
	store.Append("s", DSMessage{Role: "assistant", Content: "更晚回复"})

	store.UpsertWorkflowPair(
		"s", "wf",
		DSMessage{Role: "user", Content: "当前任务", Status: taskStatusCompleted},
		DSMessage{Role: "assistant", Content: "后来完成了"},
	)
	msgs := store.Get("s")
	if len(msgs) != 6 {
		t.Fatalf("更新工作流消息不应影响其他往返，实得 %d 条", len(msgs))
	}
	if msgs[0].Content != "更早任务" || msgs[2].Status != taskStatusCompleted ||
		msgs[3].Content != "后来完成了" || msgs[4].Content != "更晚任务" {
		t.Fatalf("原位更新破坏了消息顺序: %+v", msgs)
	}
}

func TestUpsertWorkflowPairDoesNotDowngradeCompletedRun(t *testing.T) {
	withIsolatedSessionDir(t)
	store := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(store) })
	store.UpsertWorkflowPair(
		"s", "wf",
		DSMessage{Role: "user", Content: "任务", Status: taskStatusCompleted},
		DSMessage{Role: "assistant", Content: "已经完成"},
	)
	store.UpsertWorkflowPair(
		"s", "wf",
		DSMessage{Role: "user", Content: "任务", Status: taskStatusInterrupted},
		DSMessage{Role: "assistant", Content: "迟到的旧请求中断"},
	)
	msgs := store.Get("s")
	if len(msgs) != 2 || msgs[0].Status != taskStatusCompleted || msgs[1].Content != "已经完成" {
		t.Fatalf("迟到的中断状态不应覆盖完成态: %+v", msgs)
	}
}
