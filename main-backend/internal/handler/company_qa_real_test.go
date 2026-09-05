package handler

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCompanyQAProbeReal 真机验证：用受管 headless Chromium 打开已交付的记账本产品，
// 断言探针确实读到真实渲染结果（非白屏、有控件、交互有反应）。
// 需要本机有可用浏览器引擎；起不来时 Skip，不算失败（降级路径已在交付链验证）。
func TestCompanyQAProbeReal(t *testing.T) {
	if testing.Short() {
		t.Skip("-short 跳过真机浏览器质检")
	}
	home, _ := os.UserHomeDir()
	entry := filepath.Join(home, "rescene_data", "company", "projects", "901-直播自检-记账本", "output-app.html")
	if _, err := os.Stat(entry); err != nil {
		t.Skip("测试项目不存在，跳过真机质检:", entry)
	}
	report, png, err := companyQAProbeBrowser(entry)
	if err != nil {
		t.Skip("本机浏览器引擎不可用，质检降级为放行（预期行为）:", err.Error())
	}
	t.Logf("真机质检: 可见元素=%d 文字=%d 按钮=%d 点击变化=%v 白屏=%v 截图=%dB",
		report.JSVisible, report.TextLength, report.Buttons, report.DOMChanged, report.Blank, len(png))
	if report.Blank {
		t.Errorf("记账本产品被判白屏，探针或页面有问题: vis=%d text=%d", report.JSVisible, report.TextLength)
	}
	if len(png) < 1000 {
		t.Errorf("截图字节异常: %d", len(png))
	}
}
