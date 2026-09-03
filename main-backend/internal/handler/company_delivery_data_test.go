package handler

import (
	"reflect"
	"testing"
)

// TestDeliveryParseDataRow 验证「研究数据」xlsx 行解析：连续项目、竖线、冒号、纯文本兜底都不塞进 markdown 乱码。
func TestDeliveryParseDataRow(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"计时器|高精度番茄计时|必备", []string{"计时器", "高精度番茄计时", "必备"}},
		{"待办分组|支持分组增删|可选", []string{"待办分组", "支持分组增删", "可选"}},
		{"本地持久化：localStorage 存储", []string{"本地持久化", "localStorage 存储", "必备"}},
		{"纯文本功能点", []string{"纯文本功能点", "-", "必备"}},
	}
	for _, c := range cases {
		got := deliveryParseDataRow(c.in, 3)
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("in=%q got=%v want=%v", c.in, got, c.want)
		}
	}
}

// TestDeliverySplitDataLines 验证能跳过 markdown 标题/分隔线/列表记号，不把 # ** --- 带进 xlsx。
func TestDeliverySplitDataLines(t *testing.T) {
	in := "# Rescene AI 研究数据\n---\n**计时器**|高精度|必备\n**待办**|分组管理|可选\n1. 统计数据|周报|可选\n"
	lines := deliverySplitDataLines(in)
	if len(lines) != 3 {
		t.Fatalf("应得到 3 行，实际 %d: %#v", len(lines), lines)
	}
	if lines[0] != "计时器|高精度|必备" {
		t.Fatalf("第一行应去掉加粗，实际 %q", lines[0])
	}
}
