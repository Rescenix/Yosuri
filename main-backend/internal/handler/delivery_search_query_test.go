package handler

import (
	"strings"
	"testing"
)

// TestDeliverySearchQuery 检索词清洗：整句指令 → 紧凑搜索词（不拆成单字）。
func TestDeliverySearchQuery(t *testing.T) {
	cases := []struct {
		brief   string
		mustIn  []string // 结果必须包含这些词
		mustNot []string // 结果绝不能出现这些残留
	}{
		{
			brief:  "做一个学生专注冲刺台：番茄钟功能,要能运行,含计时器与待办分组",
			mustIn: []string{"学生专注冲刺台", "番茄钟", "计时器", "待办分组"},
		},
		{
			brief:   "帮我做一个记账本，要能记收支，按月统计",
			mustIn:  []string{"记账本", "收支", "按月统计"},
			mustNot: []string{"帮我做", "要能"},
		},
		{
			brief:  "开发一个密码生成器",
			mustIn: []string{"密码生成器"},
		},
	}
	for _, c := range cases {
		q := deliverySearchQuery(c.brief)
		if len([]rune(q)) > 60 {
			t.Errorf("检索词过长: %q", q)
		}
		for _, w := range c.mustIn {
			if !strings.Contains(q, w) {
				t.Errorf("检索词丢失关键产品词 %q → %q", w, q)
			}
		}
		for _, w := range c.mustNot {
			if strings.Contains(q, w) {
				t.Errorf("检索词残留语气词 %q → %q", w, q)
			}
		}
		t.Logf("brief=%q → q=%q", c.brief, q)
	}
}
