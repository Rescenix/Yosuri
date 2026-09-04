package handler

import (
	"strings"
	"testing"
)

// TestCompanyDecidePlan 选型规则：状态/存储类走多文件，小工具走单文件。
func TestCompanyDecidePlan(t *testing.T) {
	cases := []struct {
		brief string
		multi bool
	}{
		{"做一个记账本，能记录收支、按月统计", true},
		{"做一个待办清单应用，支持登录和分类管理", true},
		{"做一个番茄钟小工具，含25分钟计时器", false},
		{"做一个颜色换算工具", false},
		{"做一个个人博客系统", true},
	}
	for _, c := range cases {
		got := companyDecidePlan(c.brief)
		if got.MultiFile != c.multi {
			t.Errorf("选型错误 %q: got MultiFile=%v want %v (reason=%s)", c.brief, got.MultiFile, c.multi, got.Reason)
		}
	}
}

// TestCompanyInlineMulti 内联装配：link/script src 全部替换成内联块。
func TestCompanyInlineMulti(t *testing.T) {
	files := companyMultiFiles{
		"index.html": `<!doctype html><html><head><link rel="stylesheet" href="styles.css"></head><body><script src="data.js"></script><script src="app.js"></script></body></html>`,
		"styles.css": "body{margin:0}",
		"data.js":    "window.AppData={}",
		"app.js":     "console.log(1)",
	}
	out := companyInlineMulti(files)
	for _, want := range []string{"<style>", "body{margin:0}", "window.AppData", "console.log(1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("内联结果缺 %q", want)
		}
	}
	for _, bad := range []string{`href="styles.css"`, `src="data.js"`, `src="app.js"`} {
		if strings.Contains(out, bad) {
			t.Errorf("内联结果仍残留外链 %q", bad)
		}
	}
}

// TestCompanyMultiTemplates 兜底模板自身合法：data.js 有 localStorage、app.js 有 AppData、css 有 @media。
func TestCompanyMultiTemplates(t *testing.T) {
	if !strings.Contains(companyDataJSTemplate("测试"), "localStorage") {
		t.Error("data.js 兜底缺 localStorage")
	}
	if !strings.Contains(companyAppJSTemplate("测试"), "AppData") {
		t.Error("app.js 兜底缺 AppData")
	}
	if !strings.Contains(companyCSSFallback(), "@media") {
		t.Error("css 兜底缺 @media")
	}
}
