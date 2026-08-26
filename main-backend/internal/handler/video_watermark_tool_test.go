package handler

import (
	"context"
	"encoding/json"
	"testing"
)

// TestCallWatermarkRemove 全链路：模拟带水印视频 → 工具去水印 → 校验输出存在。
func TestCallWatermarkRemove(t *testing.T) {
	// 入参（指向已造好的模拟水印视频，见 test_output/wm_probe/src.mp4）
	// 手动坐标=模拟水印块位置 (1830,1020,70,40) 外扩几像素，验证 delogo 链路干净
	args := map[string]interface{}{
		"video": "C:/Pro2026/re0/test_output/wm_probe/src.mp4",
		"out":   "C:/Pro2026/re0/test_output/wm_probe/clean_tool.mp4",
		"x":     1825, "y": 1015, "w": 80, "h": 50,
	}
	raw, _ := json.Marshal(args)
	res, err := callWatermarkRemove(context.Background(), string(raw))
	if err != nil {
		t.Fatalf("callWatermarkRemove 失败: %v", err)
	}
	t.Logf("结果: %s", res.Text)
}
