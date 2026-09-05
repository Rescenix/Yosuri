package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCompanyQAE2EReal 端到端真跑一条指令，验证质检员确实嵌进了交付链。
func TestCompanyQAE2EReal(t *testing.T) {
	if testing.Short() {
		t.Skip("-short 跳过真实生产")
	}
	name := "902-质检自检-便签清单"
	dir, err := deliveryBuildProject(name, "做一个便签清单应用，能新增便签、删除便签、按颜色分类，数据要本地保存刷新不丢")
	if err != nil {
		t.Fatalf("真实生产失败: %v", err)
	}

	data, readErr := os.ReadFile(filepath.Join(dir, "09-质量验收.qa.json"))
	if readErr != nil {
		t.Fatalf("质检报告未落盘（质检员没接进交付链？）: %v", readErr)
	}
	var rep companyQAReport
	if json.Unmarshal(data, &rep) != nil {
		t.Fatalf("质检报告不是合法 JSON")
	}
	t.Logf("质检结论: passed=%v 白屏=%v 可见=%d 文字=%d 按钮=%d 点击=%d 视觉=%d 返修=%v",
		rep.Passed, rep.Blank, rep.JSVisible, rep.TextLength, rep.Buttons, rep.Clicked, rep.VisualScore, rep.Repaired)
	t.Logf("缺失功能数=%d 清单=%v", rep.MissingFeatureCount, rep.MissingFeatures)
	t.Logf("摘要=%s", rep.Summary)
	// 最后删测试项目
	_ = os.RemoveAll(dir)
}
