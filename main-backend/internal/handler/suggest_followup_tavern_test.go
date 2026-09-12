package handler

import (
	"fmt"
	"testing"
	"time"
)

// 酒馆 RP 场景：没有工具调用，动作记录为空——之前必现"没看到记录拒绝生成"。
// go test -run TestSuggestFollowUpTavern -v -timeout 60s
func TestSuggestFollowUpTavern(t *testing.T) {
	start := time.Now()
	got := suggestFollowUp(
		"猫娘角色扮演，用户今天心情不好，想被人安慰",
		"（猫娘语气）主人今天一定很累了吧…那喵，我陪你说说话，摸摸头，不开心的事都会过去的喵～",
		nil, // 酒馆：没有工具调用，transcript 为空
	)
	elapsed := time.Since(start)
	t.Logf("耗时: %v, 建议: %v", elapsed, got)
	if len(got) == 0 {
		t.Fatalf("酒馆场景建议为空（耗时 %v）——空动作记录仍导致拒答", elapsed)
	}
	if len(got) != 3 {
		t.Errorf("期望恰好 3 条，实际 %d 条: %v", len(got), got)
	}
	fmt.Printf("✅ 酒馆建议 %d 条 / %v\n", len(got), elapsed)
	for i, s := range got {
		fmt.Printf("   %d. %s\n", i+1, s)
	}
}