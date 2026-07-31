package handler

// 工具级并行：把一轮里的多个工具调用切成「并行段 + 顺序段」，读类并发、写类屏障。
//
// 对齐 Hermes 的分段规划思想，按 re0 现状精简：
//   - 并行候选：只读（isReadOnlyToolCall）且不越界（!toolOutsideRoot）且非 dispatch_agent
//   - 屏障：写类工具、run_command、越界访问、dispatch_agent（它自带子代理并行，单独处理）
//   - 路径冲突：并行候选的读目标与「已计划」的写目标重叠 → 降为顺序段
//
// 为什么并行候选天然免审批：approval.go 里只读且不越界的调用直接放行
// （maybeRequestApproval 短路返回 true），所以并发执行不会触发 waiter 阻塞，
// 审批语义保持单线程，SSE 事件顺序也不乱。
//
// 顺序保证：段与段之间严格保持模型原始顺序；并行段内部无副作用冲突，
// 结果在 executeCodeCalls 里按原始索引收集，模型看到的 tool result 顺序不变。

import (
	"path/filepath"
	"strings"

	"backend/internal/ai/core"
)

// toolSegment 一段工具调用。parallel=true 时段内调用可并发执行。
type toolSegment struct {
	parallel bool
	calls    []core.ToolCall
}

// planParallelSegments 把一批工具调用切成并行/顺序段，保持模型原始顺序。
func planParallelSegments(calls []core.ToolCall) []toolSegment {
	var segs []toolSegment
	var current []core.ToolCall
	// 已计划（出现在当前及之前屏障段）的写路径，用于 read 的冲突检测
	var writePaths []string

	flush := func() {
		if len(current) < 2 {
			// 单个调用不值得开并发：降级顺序，走现有单线程分发
			for _, tc := range current {
				segs = append(segs, toolSegment{parallel: false, calls: []core.ToolCall{tc}})
			}
		} else {
			segs = append(segs, toolSegment{parallel: true, calls: current})
		}
		current = nil
	}

	for _, tc := range calls {
		name := tc.Function.Name
		args := tc.Function.Arguments
		if name == "dispatch_agent" {
			// 子代理自带 goroutine 并行（executeCodeCalls 既有逻辑），
			// 在这里是屏障，避免与段内并发混在一起。
			flush()
			segs = append(segs, toolSegment{parallel: false, calls: []core.ToolCall{tc}})
			continue
		}
		if outside, _ := toolOutsideRoot(args); isReadOnlyToolCall(name, args) && !outside {
			if p := firstToolPath(args); p != "" && pathOverlapsAny(p, writePaths) {
				// 读目标与前面的写目标重叠：必须在写之后执行，保持顺序
				flush()
				segs = append(segs, toolSegment{parallel: false, calls: []core.ToolCall{tc}})
				continue
			}
			current = append(current, tc)
			continue
		}
		// 写类 / 需审批 / 越界 → 屏障
		flush()
		if p := firstToolPath(args); p != "" {
			writePaths = append(writePaths, p)
		}
		segs = append(segs, toolSegment{parallel: false, calls: []core.ToolCall{tc}})
	}
	flush()
	return segs
}

// firstToolPath 取工具参数里的第一个路径（路径冲突检测用，取一个就够）。
func firstToolPath(argsJSON string) string {
	paths := toolPathArgs(argsJSON)
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// pathOverlapsAny 判断路径 p 是否与任一已知写路径落在同一子树。
// 复用 approval.go 的规范化（absAgainstRoot + normCase），大小写不敏感。
func pathOverlapsAny(p string, writePaths []string) bool {
	if len(writePaths) == 0 {
		return false
	}
	canon := normCase(absAgainstRoot(p))
	for _, w := range writePaths {
		wc := normCase(absAgainstRoot(w))
		if canonicalOverlap(canon, wc) {
			return true
		}
	}
	return false
}

// canonicalOverlap 判断两个规范化绝对路径是否互相是前缀/子树关系。
func canonicalOverlap(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	sep := string(filepath.Separator)
	as := strings.TrimSuffix(a, sep)
	bs := strings.TrimSuffix(b, sep)
	return as == bs || strings.HasPrefix(as, bs+sep) || strings.HasPrefix(bs, as+sep)
}
