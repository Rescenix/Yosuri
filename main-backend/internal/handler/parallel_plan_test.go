package handler

import (
	"testing"

	"backend/internal/ai/core"
)

func tc(name, args string) core.ToolCall {
	return core.ToolCall{Function: core.ToolCallFunc{Name: name, Arguments: args}}
}

// segKinds 提取段类型序列，便于断言
func segKinds(segs []toolSegment) []bool {
	out := make([]bool, len(segs))
	for i, s := range segs {
		out[i] = s.parallel
	}
	return out
}

func segCount(segs []toolSegment) int {
	n := 0
	for _, s := range segs {
		n += len(s.calls)
	}
	return n
}

// 全部只读 → 单并行段
func TestPlanParallelAllReads(t *testing.T) {
	calls := []core.ToolCall{
		tc("read_file", `{"path":"a.go"}`),
		tc("grep", `{"pattern":"foo"}`),
		tc("web_search", `{"query":"go"}`),
		tc("view_image", `{"path":"a.png"}`),
	}
	segs := planParallelSegments(calls)
	if len(segs) != 1 || !segs[0].parallel {
		t.Fatalf("期望 1 个并行段，得到 %v", segKinds(segs))
	}
	if segCount(segs) != 4 {
		t.Fatalf("段内应含 4 个调用，得到 %d", segCount(segs))
	}
}

// 写工具 → 屏障，前后的连续读各自成并行段
func TestPlanWriteBarrier(t *testing.T) {
	calls := []core.ToolCall{
		tc("read_file", `{"path":"a.go"}`),
		tc("read_file", `{"path":"b.go"}`),
		tc("write_file", `{"path":"c.go","content":"x"}`),
		tc("read_file", `{"path":"d.go"}`),
		tc("read_file", `{"path":"e.go"}`),
	}
	segs := planParallelSegments(calls)
	// [并行 read a,b] → [顺序 write c] → [并行 read d,e]
	if len(segs) != 3 {
		t.Fatalf("期望 3 段，得到 %v", segKinds(segs))
	}
	if !segs[0].parallel || segs[1].parallel || !segs[2].parallel {
		t.Fatalf("段类型错误: %v", segKinds(segs))
	}
	if len(segs[0].calls) != 2 || len(segs[2].calls) != 2 {
		t.Fatalf("并行段应各含 2 个调用: %v", segKinds(segs))
	}
}

// read 与前面 write 同路径 → 降为顺序，保证 read 读到写后内容
func TestPlanReadAfterWriteSamePath(t *testing.T) {
	calls := []core.ToolCall{
		tc("write_file", `{"path":"a.go","content":"new"}`),
		tc("read_file", `{"path":"a.go"}`),
	}
	segs := planParallelSegments(calls)
	for i, s := range segs {
		if s.parallel {
			t.Fatalf("段 %d 不应并行（read 依赖 write 结果）: %v", i, segKinds(segs))
		}
	}
	if segCount(segs) != 2 {
		t.Fatalf("应保留 2 个调用，得到 %d", segCount(segs))
	}
}

// 子目录也算路径重叠（read a/b.go 与 write a 同子树）
func TestPlanSubtreeOverlap(t *testing.T) {
	calls := []core.ToolCall{
		tc("write_file", `{"path":"src/","content":"x"}`),
		tc("read_file", `{"path":"src/main.go"}`),
	}
	segs := planParallelSegments(calls)
	for i, s := range segs {
		if s.parallel {
			t.Fatalf("段 %d 不应并行（子树重叠）: %v", i, segKinds(segs))
		}
	}
}

// dispatch_agent 是屏障（它自带子代理并行），前后读拆段
func TestPlanDispatchAgentBarrier(t *testing.T) {
	calls := []core.ToolCall{
		tc("read_file", `{"path":"a.go"}`),
		tc("dispatch_agent", `{"task":"x"}`),
		tc("read_file", `{"path":"b.go"}`),
	}
	segs := planParallelSegments(calls)
	if len(segs) != 3 {
		t.Fatalf("期望 3 段，得到 %v", segKinds(segs))
	}
	if segs[1].parallel {
		t.Fatalf("dispatch_agent 段不应并行")
	}
}

// 非只读 run_command → 屏障
func TestPlanCommandBarrier(t *testing.T) {
	calls := []core.ToolCall{
		tc("read_file", `{"path":"a.go"}`),
		tc("read_file", `{"path":"b.go"}`),
		tc("run_command", `{"command":"go build ./..."}`),
		tc("read_file", `{"path":"c.go"}`),
		tc("read_file", `{"path":"d.go"}`),
	}
	segs := planParallelSegments(calls)
	if len(segs) != 3 || !segs[0].parallel || segs[1].parallel || !segs[2].parallel {
		t.Fatalf("段类型错误: %v", segKinds(segs))
	}
}

// 单个并行候选降级顺序（不足 2 个不开并发）
func TestPlanSingleReadDemoted(t *testing.T) {
	calls := []core.ToolCall{
		tc("read_file", `{"path":"a.go"}`),
		tc("write_file", `{"path":"b.go","content":"x"}`),
	}
	segs := planParallelSegments(calls)
	for i, s := range segs {
		if s.parallel {
			t.Fatalf("段 %d 不应并行（单候选降级）: %v", i, segKinds(segs))
		}
	}
	if segCount(segs) != 2 {
		t.Fatalf("应保留 2 个调用，得到 %d", segCount(segs))
	}
}

// 顺序保持：模型原始顺序在任何分段下都不变
func TestPlanPreservesOrder(t *testing.T) {
	calls := []core.ToolCall{
		tc("read_file", `{"path":"a.go"}`),
		tc("write_file", `{"path":"b.go","content":"x"}`),
		tc("grep", `{"pattern":"y"}`),
		tc("edit_file", `{"path":"c.go","old_string":"o","new_string":"n"}`),
		tc("glob", `{"pattern":"**/*.go"}`),
	}
	segs := planParallelSegments(calls)
	var names []string
	for _, s := range segs {
		for _, c := range s.calls {
			names = append(names, c.Function.Name)
		}
	}
	want := []string{"read_file", "write_file", "grep", "edit_file", "glob"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("顺序被破坏: 得到 %v 期望 %v", names, want)
		}
	}
}
