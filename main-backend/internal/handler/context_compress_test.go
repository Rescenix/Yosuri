package handler

import (
	"context"
	"encoding/json"
	"testing"
)

// 构造一段典型的工作流 msgs：system + task + N 轮(assistant带tool_calls + tool结果)
func buildRounds(task string, rounds int) []map[string]any {
	msgs := []map[string]any{
		{"role": "system", "content": "你是 agent"},
		{"role": "user", "content": task},
	}
	for i := range rounds {
		callID := "call_" + string(rune('a'+i))
		msgs = append(msgs, map[string]any{
			"role": "assistant", "content": "第" + string(rune('0'+i)) + "轮",
			"tool_calls": []map[string]any{
				{"id": callID, "type": "function", "function": map[string]any{
					"name": "read_file", "arguments": `{"path":"x.go"}`}}},
		})
		msgs = append(msgs, map[string]any{
			"role": "tool", "tool_call_id": callID, "content": "文件内容若干",
		})
	}
	return msgs
}

func TestCompressSkippedUnderThreshold(t *testing.T) {
	r := &WorkflowRunner{}
	msgs := buildRounds("任务A", 10)
	// promptTokens 远低于 80% 窗口 → 不该压
	out, cr := r.compressContextIfNeeded(context.TODO(), nil, msgs, "任务A", 1000, 128000)
	if cr.Compressed {
		t.Error("未到阈值不该压缩")
	}
	if len(out) != len(msgs) {
		t.Error("未触发时应原样返回")
	}
}

// 定位辅助：任务消息绝不能落进被压缩的中段。
func TestTaskMessageNeverInMiddle(t *testing.T) {
	msgs := buildRounds("关键任务", 6)
	taskIdx := lastIndexOfTask(msgs, "关键任务")
	if taskIdx != 1 {
		t.Fatalf("任务消息应在下标 1，实得 %d", taskIdx)
	}
	tailStart := roundStartFromEnd(msgs, compressKeepRecentRounds)
	if tailStart <= taskIdx {
		t.Fatal("保留尾巴的起点不该盖到任务消息")
	}
}

// 折叠边界必须落在整轮上——middle 的两端都应是 assistant 轮首，
// 这样替换成一条摘要后不会留下"有结果没调用/有调用没结果"的孤儿。
func TestRoundStartFromEndLandsOnAssistant(t *testing.T) {
	msgs := buildRounds("t", 6)
	for keep := 1; keep <= 6; keep++ {
		idx := roundStartFromEnd(msgs, keep)
		if idx >= len(msgs) {
			t.Fatalf("keep=%d 找不到轮首", keep)
		}
		if msgs[idx]["role"] != "assistant" || len(toolCallNames(msgs[idx])) == 0 {
			t.Errorf("keep=%d 的轮首下标 %d 不是带 tool_calls 的 assistant", keep, idx)
		}
	}
}

// tool_calls 经过检查点 JSON 往返会变成 []interface{}，轮首检测必须仍然认得。
func TestToolCallNamesSurvivesJSONRoundTrip(t *testing.T) {
	msgs := buildRounds("t", 2)
	data, _ := json.Marshal(msgs)
	var restored []map[string]any
	json.Unmarshal(data, &restored)

	// 内存态（[]map[string]any）
	if len(toolCallNames(msgs[2])) != 1 {
		t.Error("内存态 assistant 的工具名没取到")
	}
	// JSON 往返态（[]interface{}）——不兼容的话续跑后压缩的轮界检测会失效
	if len(toolCallNames(restored[2])) != 1 {
		t.Error("JSON 往返后 assistant 的工具名没取到，续跑压缩会错乱")
	}
	if toolCallNames(restored[2])[0] != "read_file" {
		t.Errorf("工具名解析错误: %v", toolCallNames(restored[2]))
	}
}

// 一条没有 tool_calls 的普通消息不该被误判为轮首。
func TestToolCallNamesEmptyForPlainMessage(t *testing.T) {
	if n := toolCallNames(map[string]any{"role": "user", "content": "hi"}); len(n) != 0 {
		t.Errorf("普通消息不该有工具名: %v", n)
	}
	if n := toolCallNames(map[string]any{"role": "assistant", "content": "答案", "tool_calls": []map[string]any{}}); len(n) != 0 {
		t.Errorf("空 tool_calls 不该算轮首: %v", n)
	}
}

// 端到端：喂一个假的压缩后端，验证 middle 被折叠、task 与最近轮保留、tool 配对不破。
func TestCompressFoldsMiddleKeepsTaskAndRecent(t *testing.T) {
	// 假后端：非流式返回固定摘要
	srv := fakeBackend(t, 200, `{"choices":[{"message":{"content":"摘要：读过 x.go，待办改 y"}}]}`)
	defer srv.Close()
	backends := []RouterBackend{{Name: "压缩", BaseURL: srv.URL, Model: "m", APIKey: "k"}}

	r := &WorkflowRunner{}
	msgs := buildRounds("我的任务", 8) // 2 + 8*2 = 18 条
	// promptTokens 拉高到超阈值
	out, cr := r.compressContextIfNeeded(context.TODO(), backends, msgs, "我的任务", 200000, 128000)

	if !cr.Compressed {
		t.Fatal("超阈值应触发压缩")
	}
	if len(out) >= len(msgs) {
		t.Errorf("压缩后消息数应减少：%d -> %d", len(msgs), len(out))
	}
	// 头两条（system + task）必须原样在前
	if out[0]["role"] != "system" || out[1]["content"] != "我的任务" {
		t.Error("system / task 前缀被破坏")
	}
	// 摘要消息必须出现且带标记
	foundSummary := false
	for _, m := range out {
		if c, _ := m["content"].(string); len(c) >= len(compressMarker) && c[:len(compressMarker)] == compressMarker {
			foundSummary = true
		}
	}
	if !foundSummary {
		t.Error("没找到带标记的摘要消息")
	}
	// 最近若干轮的 tool 结果应仍在（保留原文）
	last := out[len(out)-1]
	if last["role"] != "tool" {
		t.Errorf("最后一条应是最近一轮的 tool 结果，实得 %v", last["role"])
	}
	// 关键：不能留下孤儿——每个 tool 结果都能往前找到配对的 assistant tool_calls
	assertToolPairing(t, out)
}

// 二次压缩：上一版摘要应被重新折叠，而不是越堆越多。
func TestSecondCompressionFoldsPriorSummary(t *testing.T) {
	srv := fakeBackend(t, 200, `{"choices":[{"message":{"content":"新摘要"}}]}`)
	defer srv.Close()
	backends := []RouterBackend{{Name: "压缩", BaseURL: srv.URL, Model: "m", APIKey: "k"}}
	r := &WorkflowRunner{}

	msgs := buildRounds("任务X", 8)
	once, _ := r.compressContextIfNeeded(context.TODO(), backends, msgs, "任务X", 200000, 128000)
	// 再追加几轮，再压一次
	once = append(once, buildRounds("任务X", 5)[2:]...) // 复用轮次结构，去掉重复的 system+task
	twice, cr := r.compressContextIfNeeded(context.TODO(), backends, once, "任务X", 200000, 128000)

	if !cr.Compressed {
		t.Fatal("二次应触发")
	}
	// 摘要消息只能有一条——旧摘要被折进新摘要
	count := 0
	for _, m := range twice {
		if c, _ := m["content"].(string); len(c) >= len(compressMarker) && c[:len(compressMarker)] == compressMarker {
			count++
		}
	}
	if count != 1 {
		t.Errorf("压缩摘要应始终只有一条，实得 %d 条（旧摘要没被折叠）", count)
	}
}

// assertToolPairing 校验：每条 role:tool 的前面（跳过连续的 tool）必有一条带 tool_calls 的 assistant。
func assertToolPairing(t *testing.T, msgs []map[string]any) {
	t.Helper()
	for i, m := range msgs {
		if m["role"] != "tool" {
			continue
		}
		// 往前找最近的非 tool 消息，必须是带 tool_calls 的 assistant
		j := i - 1
		for j >= 0 && msgs[j]["role"] == "tool" {
			j--
		}
		if j < 0 || msgs[j]["role"] != "assistant" || len(toolCallNames(msgs[j])) == 0 {
			t.Errorf("下标 %d 的 tool 结果是孤儿（前面没有配对的 assistant tool_calls）", i)
		}
	}
}
