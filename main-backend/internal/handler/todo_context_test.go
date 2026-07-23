package handler

import (
	"strings"
	"testing"
)

// 没有清单时不能凭空注入一段（空串 = 调用方不追加这条消息）。
func TestTodoContextLineEmpty(t *testing.T) {
	if got := todoContextLine(nil); got != "" {
		t.Errorf("无清单时不该产出内容: %q", got)
	}
	if got := todoContextLine([]todoItem{}); got != "" {
		t.Errorf("空清单时不该产出内容: %q", got)
	}
}

// 清单要带状态和进度：模型据此判断"哪些别重做、下一步做哪个"。
func TestTodoContextLineRendersStateAndProgress(t *testing.T) {
	got := todoContextLine([]todoItem{
		{Text: "写 SkillWall 组件", Status: "done"},
		{Text: "加标签筛选", Status: "doing"},
		{Text: "补空状态占位", Status: "pending"},
	})

	if !strings.Contains(got, "权威状态") {
		t.Error("没声明这是权威状态，模型可能仍以自己的记忆为准")
	}
	for _, want := range []string{"写 SkillWall 组件", "加标签筛选", "补空状态占位"} {
		if !strings.Contains(got, want) {
			t.Errorf("清单项丢失: %s", want)
		}
	}
	if !strings.Contains(got, "☑") || !strings.Contains(got, "▶") || !strings.Contains(got, "☐") {
		t.Errorf("三种状态没有被区分开:\n%s", got)
	}
	if !strings.Contains(got, "1/3") {
		t.Errorf("进度不对（1 个 done / 共 3 个）:\n%s", got)
	}
	if !strings.Contains(got, "别重做已完成项") {
		t.Error("没告诉模型别重做已完成项——这正是要防的失败模式")
	}
}

// Todos 必须能穿过检查点往返，否则续跑后计划照样丢。
func TestTodosSurviveCheckpointRoundTrip(t *testing.T) {
	withTempDataDir(t)

	saveWorkflowCheckpoint(&workflowCheckpoint{
		WorkflowID: "wf_todo", SessionID: "s1", Task: "t",
		Todos: []todoItem{
			{Text: "第一步", Status: "done"},
			{Text: "第二步", Status: "doing"},
		},
	})

	got := loadWorkflowCheckpoint("wf_todo")
	if got == nil {
		t.Fatal("检查点没读回来")
	}
	if len(got.Todos) != 2 {
		t.Fatalf("清单条数不对: %d", len(got.Todos))
	}
	if got.Todos[0].Text != "第一步" || got.Todos[0].Status != "done" {
		t.Errorf("清单内容/状态丢失: %+v", got.Todos)
	}
}

// 旧检查点没有 todos 字段，读回来必须是空而不是报错——存量数据不能因此打不开。
func TestCheckpointWithoutTodosStillLoads(t *testing.T) {
	withTempDataDir(t)
	saveWorkflowCheckpoint(&workflowCheckpoint{WorkflowID: "wf_legacy", Task: "t"})
	got := loadWorkflowCheckpoint("wf_legacy")
	if got == nil {
		t.Fatal("无 todos 的检查点读不回来了")
	}
	if len(got.Todos) != 0 {
		t.Errorf("不该凭空多出清单: %+v", got.Todos)
	}
}
