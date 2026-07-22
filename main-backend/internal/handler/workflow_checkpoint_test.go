package handler

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 所有用例都把 SHANXI_DATA_DIR 指到临时目录，绝不碰用户真实的 ~/shanxi_data。
func withTempDataDir(t *testing.T) {
	t.Helper()
	t.Setenv("SHANXI_DATA_DIR", t.TempDir())
}

func TestCheckpointRoundTrip(t *testing.T) {
	withTempDataDir(t)

	cp := &workflowCheckpoint{
		WorkflowID: "wf_1", SessionID: "s1", Task: "读一下 hello.py",
		Mode: "yolo", Model: "free_ollama_cloud_gpt_oss_120b", Round: 3,
		Msgs: []map[string]any{
			{"role": "system", "content": "你是..."},
			{"role": "assistant", "content": "", "tool_calls": []map[string]any{
				{"id": "call_1", "type": "function", "function": map[string]any{
					"name": "read_text_file", "arguments": `{"path":"hello.py"}`}}}},
			{"role": "tool", "tool_call_id": "call_1", "content": "print('我的世界')"},
		},
		Transcript:   []string{"read_text_file(...) => print('我的世界')"},
		CallSigCount: map[string]int{`read_text_file|{"path":"hello.py"}`: 1},
		CallSeq:      1, InputTokens: 1200, OutputTokens: 340,
	}
	saveWorkflowCheckpoint(cp)

	got := loadWorkflowCheckpoint("wf_1")
	if got == nil {
		t.Fatal("落盘后读不回来")
	}
	if got.Task != cp.Task || got.Round != 3 || got.CallSeq != 1 {
		t.Errorf("标量字段没对上: %+v", got)
	}
	if got.InputTokens != 1200 || got.OutputTokens != 340 {
		t.Errorf("token 统计没对上: in=%d out=%d", got.InputTokens, got.OutputTokens)
	}
	if len(got.Msgs) != 3 {
		t.Fatalf("msgs 条数 = %d，要 3", len(got.Msgs))
	}
	// 续跑的关键：assistant.tool_calls 和它配对的 tool 结果都必须活着回来，
	// 少任何一半模型都会看不见自己调用过工具（就是 Ollama 原生端点那个坑）。
	if _, ok := got.Msgs[1]["tool_calls"]; !ok {
		t.Error("assistant.tool_calls 在往返中丢失")
	}
	if got.Msgs[2]["tool_call_id"] != "call_1" {
		t.Errorf("tool 结果的 tool_call_id 丢失: %+v", got.Msgs[2])
	}
	if got.CallSigCount[`read_text_file|{"path":"hello.py"}`] != 1 {
		t.Error("熔断计数没持久化——续跑后重复调用护栏会失效")
	}
}

func TestCheckpointDeleteAndMissing(t *testing.T) {
	withTempDataDir(t)

	if loadWorkflowCheckpoint("不存在") != nil {
		t.Error("不存在的检查点应返回 nil")
	}
	saveWorkflowCheckpoint(&workflowCheckpoint{WorkflowID: "wf_2", Task: "t"})
	deleteWorkflowCheckpoint("wf_2")
	if loadWorkflowCheckpoint("wf_2") != nil {
		t.Error("删除后仍能读到")
	}
}

func TestCheckpointExpiryAndCorruption(t *testing.T) {
	withTempDataDir(t)

	// 过期：UpdatedAt 由 save 覆写，所以直接改文件模拟一个 25 小时前的检查点
	saveWorkflowCheckpoint(&workflowCheckpoint{WorkflowID: "wf_old", Task: "t"})
	path := filepath.Join(checkpointDir(), "wf_old.json")
	data, _ := os.ReadFile(path)
	stale := time.Now().Add(-25 * time.Hour).Format(time.RFC3339Nano)
	os.WriteFile(path, []byte(replaceUpdatedAt(string(data), stale)), 0o644)
	if loadWorkflowCheckpoint("wf_old") != nil {
		t.Error("超过 TTL 的检查点应被判为废弃")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("过期检查点应被顺手删除")
	}

	// 损坏：半个 JSON（进程在写盘途中被杀的场景）不能让续跑炸掉
	os.WriteFile(filepath.Join(checkpointDir(), "wf_bad.json"), []byte(`{"task":"x"`), 0o644)
	if loadWorkflowCheckpoint("wf_bad") != nil {
		t.Error("损坏的检查点应返回 nil 而不是半个状态")
	}
}

func TestListCheckpointsFiltersBySession(t *testing.T) {
	withTempDataDir(t)

	saveWorkflowCheckpoint(&workflowCheckpoint{WorkflowID: "a", SessionID: "s1", Task: "任务A"})
	saveWorkflowCheckpoint(&workflowCheckpoint{WorkflowID: "b", SessionID: "s2", Task: "任务B"})

	if n := len(listWorkflowCheckpoints("")); n != 2 {
		t.Errorf("不过滤时应列出 2 个，实得 %d", n)
	}
	only := listWorkflowCheckpoints("s1")
	if len(only) != 1 || only[0]["workflow_id"] != "a" {
		t.Errorf("按 session 过滤失败: %+v", only)
	}
}

// replaceUpdatedAt 把序列化后的 updated_at 换成指定时间戳（仅测试用）。
func replaceUpdatedAt(json, ts string) string {
	i := indexOf(json, `"updated_at":"`)
	if i < 0 {
		return json
	}
	start := i + len(`"updated_at":"`)
	end := indexOf(json[start:], `"`)
	return json[:start] + ts + json[start+end:]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
