package handler

import "testing"

// TestCompanyQAPassLayout 布局充实度判定：高度比 <1.2 判塌陷，测不到不误杀。
func TestCompanyQAPassLayout(t *testing.T) {
	base := companyQAReport{JSVisible: 50, TextLength: 400, Buttons: 3, InteractOK: true, DOMChanged: true}
	// 高度比 0.9（塌陷）→ 不通过
	r := base
	r.LayoutOK = true
	r.PageHeightRatio = 0.9
	if companyQAPass(r) {
		t.Errorf("高度比 0.9 应判不合格")
	}
	// 高度比 1.2（临界达标）→ 通过
	r.PageHeightRatio = 1.2
	if !companyQAPass(r) {
		t.Errorf("高度比 1.2 应判合格")
	}
	// 没测到高度比（LayoutOK=false）→ 不据此判（铁律：没测到≠不合格）
	r2 := base
	r2.LayoutOK = false
	r2.PageHeightRatio = 0.5
	if !companyQAPass(r2) {
		t.Errorf("未测到高度比不应据此判失败")
	}
}
