package handler

// 验证 run_task 生命周期事件：启动成功必须推 start 事件（前端据此登记 running 卡片），
// 进程退出再推 done 事件。修复前只有 done、没有 start → 前端运行期间面板空白。
import (
	"testing"
	"time"
)

func TestStartBgTaskEmitsStartAndDoneEvents(t *testing.T) {
	ch := make(chan bgTaskResult, 4)
	id, err := startBgTask("test-wf", "echo bg-task-event-test", ch)
	if err != nil {
		t.Fatalf("startBgTask 失败: %v", err)
	}

	// 1) 必须先收到 start 事件
	select {
	case res := <-ch:
		if res.Stage != "start" {
			t.Fatalf("首个事件应为 start，实际 stage=%q", res.Stage)
		}
		if res.TaskID != id {
			t.Fatalf("start 事件 task_id 不匹配: %s != %s", res.TaskID, id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("5s 内没收到 start 事件——run_task 启动不推事件，前端永远看不到运行中卡片")
	}

	// 2) 进程退出后必须收到 done 事件
	select {
	case res := <-ch:
		if res.Stage != "done" {
			t.Fatalf("第二个事件应为 done，实际 stage=%q", res.Stage)
		}
		if res.ExitCode != 0 {
			t.Fatalf("echo 应退出码 0，实际 %d", res.ExitCode)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("10s 内没收到 done 事件")
	}

	// 3) 清理：确保测试任务不残留
	killBgTask(id)
}
