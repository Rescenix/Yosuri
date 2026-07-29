package handler

import (
	"context"
	"testing"
)

func TestStopWorkflowCancelsRegisteredContext(t *testing.T) {
	ctx, control := registerWorkflowControl(context.Background(), "wf_stop_test")
	t.Cleanup(func() { unregisterWorkflowControl("wf_stop_test", control) })

	got, ok := stopWorkflow("wf_stop_test")
	if !ok || got != control {
		t.Fatal("已注册工作流无法停止")
	}
	if !control.stopped.Load() {
		t.Fatal("停止原因没有被记录")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("停止没有取消工作流 context")
	}
}

func TestStopWorkflowRejectsUnknownID(t *testing.T) {
	if _, ok := stopWorkflow("definitely_missing_workflow"); ok {
		t.Fatal("不存在的工作流不应报告停止成功")
	}
}
