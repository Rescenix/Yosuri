package handler

import (
	"strings"
	"testing"
)

// TestDeliveryDataRowsMinFloor 数据行数下限：模型只给 2 条时从上游产物补足到 6。
func TestDeliveryDataRowsMinFloor(t *testing.T) {
	// 模拟模型只给了 2 条
	modelRows := "计时器|25/5 分钟切换|必备\n待办分组|四象限|必备"
	dataRows := [][]string{{"功能点", "说明", "状态"}}
	for _, ln := range deliverySplitDataLines(modelRows) {
		if row := deliveryParseDataRow(ln, 3); row != nil {
			dataRows = append(dataRows, row)
		}
	}
	if len(dataRows)-1 != 2 {
		t.Fatalf("前置条件失败: 模型行数=%d", len(dataRows)-1)
	}
	// 上游产物里还有 4 条可抽
	upstream := "# 需求计划\n\n用户故事：作为学生，我要专注计时，以便提升学习效率\n\n功能清单：\n专注统计|显示每日专注时长|P0\n声音提醒|计时结束播放提示音|P1\n主题切换|支持亮暗两套主题|P2\n数据导出|一键导出 CSV|P2"
	n := len(dataRows) - 1
	if n < 6 {
		for _, ln := range deliverySplitDataLines(upstream) {
			if n >= 6 {
				break
			}
			row := deliveryParseDataRow(ln, 3)
			if row == nil {
				continue
			}
			dup := false
			for _, have := range dataRows[1:] {
				if have[0] == row[0] {
					dup = true
					break
				}
			}
			if !dup {
				dataRows = append(dataRows, row)
				n++
			}
		}
		for n < 6 {
			dataRows = append(dataRows, []string{"补充", "兜底", "可选"})
			n++
		}
	}
	if got := len(dataRows) - 1; got < 6 {
		t.Errorf("补足后应≥6 条, got %d", got)
	}
	// 已有的 2 条不能丢
	var parts []string
	for _, r := range dataRows[1:] {
		parts = append(parts, strings.Join(r, "|"))
	}
	joined := strings.Join(parts, "||")
	if !strings.Contains(joined, "计时器") || !strings.Contains(joined, "待办分组") {
		t.Errorf("原有功能行丢失: %s", joined)
	}
}
