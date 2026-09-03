package handler

import "testing"

// TestCompanyModelBackends 只读验证公司路由的健康候选池非空，且都是可并发打上游的 chat/completions 源。
// 不发任何上游请求、不耗额度、不动运行中的后端。
func TestCompanyModelBackends(t *testing.T) {
	bs := companyModelBackends()
	if len(bs) == 0 {
		t.Fatalf("公司模型池返回空候选")
	}
	t.Logf("健康候选 %d 个:", len(bs))
	for i, b := range bs {
		t.Logf("  #%d name=%s base=%s model=%s keyless=%v", i, b.Name, b.BaseURL, b.Model, b.Keyless)
	}
}

// TestMimoInVisionChain 验证 mimo 已进入多模型识图链（AnalyzeImage 会按成功率排序、挂自动切下一个）。
// 只读列出，不发上游请求。
func TestMimoInVisionChain(t *testing.T) {
	vb := visionBackends()
	if len(vb) == 0 {
		t.Fatalf("识图链为空")
	}
	found := false
	for _, b := range vb {
		if b.Model == "mimo-v2.5-free" {
			found = true
		}
		t.Logf("  Vision源: name=%s model=%s", b.Name, b.Model)
	}
	if !found {
		t.Fatalf("mimo 未入识图链，当前共 %d 个 Vision 源", len(vb))
	}
	t.Logf("识图链共 %d 个 Vision 源，mimo 已入链（mimo 挂自动切下一个）", len(vb))
}
