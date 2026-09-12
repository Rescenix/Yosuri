package handler

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// 验证方案 B：建议用轻量模型独立生成，应该是秒级返回（不是主模型 30s 推理）。
// go test -run TestSuggestFollowUpFast -v -timeout 60s
func TestSuggestFollowUpFast(t *testing.T) {
	start := time.Now()
	got := suggestFollowUp(
		"帮我修复工作流末尾卡顿的问题",
		"已修复。根因是节流缺少尾随补刷，最后几个词没触发重渲染。已在 useAgentWorkflow.js 加了 scheduleStreamPaint 合批，构建通过。",
		[]string{"意图: 分析卡顿原因", "工具: read_file -> useAgentWorkflow.js", "工具: patch -> 加尾随补刷", "意图: 总结修复结果"},
	)
	elapsed := time.Since(start)
	t.Logf("耗时: %v, 建议: %v", elapsed, got)
	if len(got) == 0 {
		t.Fatalf("建议为空（耗时 %v）——轻量链路没输出", elapsed)
	}
	for _, s := range got {
		if strings.TrimSpace(s) == "" {
			t.Errorf("包含空建议: %q", s)
		}
	}
	fmt.Printf("✅ 建议 %d 条 / %v\n", len(got), elapsed)
}