package handler

import (
	"strings"
	"testing"
)

// TestInferFeaturesNewPatterns 新增产品模式的功能指纹：值日表/日历/笔记/密码/购物/打卡。
func TestInferFeaturesNewPatterns(t *testing.T) {
	cases := []struct {
		brief     string
		mustHave  []string // 必须推断出的功能名
	}{
		{"做一个班级值日表，能按星期分组显示值日生，能添加和删除值日安排", []string{"值日/排班分组展示", "添加值日/排班安排"}},
		{"做一个日程提醒小工具", []string{"日历/日程事件管理"}},
		{"做一个便签应用，能新建和保存便签", []string{"笔记创建与列表"}},
		{"做一个随机密码生成器，可选长度", []string{"生成逻辑与选项"}},
		{"做一个购物清单，能勾选已完成并显示合计", []string{"清单勾选与合计"}},
		{"做一个习惯打卡应用，显示连续天数", []string{"打卡与连续记录"}},
	}
	for _, c := range cases {
		got := inferFeaturesFromBrief(c.brief)
		names := map[string]bool{}
		for _, g := range got {
			names[g.name] = true
		}
		for _, want := range c.mustHave {
			if !names[want] {
				t.Errorf("brief=%q 未推断出功能 %q（got %v）", c.brief, want, keys(names))
			}
		}
	}
	// 无关指令不应误报
	none := inferFeaturesFromBrief("做一个计算圆面积的公式查询页")
	for _, g := range none {
		if strings.Contains(g.name, "值日") || strings.Contains(g.name, "打卡") {
			t.Errorf("无关指令误报: %v", g.name)
		}
	}
}

func keys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}
