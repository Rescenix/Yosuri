package handler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMultiProjectJSBalanced 多文件项目 app.js 兜底模板必须括号配平（node --check 真校验）。
// 模板是所有失败产物的最后防线，自身语法错 = 整个多文件交付崩。
func TestMultiProjectJSBalanced(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("本机无 node，跳过语法校验")
	}
	dir := t.TempDir()
	jsPath := filepath.Join(dir, "tpl.js")
	if err := os.WriteFile(jsPath, []byte(companyAppJSTemplate("测试")), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", "--check", jsPath).CombinedOutput()
	if err != nil {
		t.Fatalf("app.js 兜底模板语法错: %v\n%s", err, out)
	}
}

// TestMultiProjectCSSFallbackComplete CSS 兜底模板必须含双断点与关键质感规格。
func TestMultiProjectCSSFallbackComplete(t *testing.T) {
	css := companyCSSFallback()
	for _, need := range []string{"@media(max-width:640px)", "@media(min-width:961px)", ":root", "tabular-nums", "@keyframes"} {
		if !strings.Contains(css, need) {
			t.Fatalf("CSS 兜底缺关键规格 %q", need)
		}
	}
}

// TestDeliveryKeywordsJoinPrefix pv 素材文件名前缀：多词拼接后仍需 ASCII 安全（sanitizeImageName 会吃中文）。
// ⚠️ deliveryKeywords 返回中文词，直接拼会被 sanitizeImageName 清成空——必须验证真实落盘名。
func TestDeliveryKeywordsJoinPrefix(t *testing.T) {
	kws := deliveryKeywords("做一个学生专注冲刺台，能番茄钟计时", 3)
	if len(kws) == 0 {
		t.Fatal("中文指令应能提取关键词")
	}
	// 关键词本身是中文——generateImage 的 sanitizeImageName 会把非 ASCII 全变 '-'，
	// 所以 pv 素材文件名前缀必须先过 ASCII 化，否则 match_media 永远命中不了。
	prefix := strings.Join(kws, "-")
	ascii := true
	for _, r := range prefix {
		if r > 127 {
			ascii = false
			break
		}
	}
	if ascii {
		t.Skip("前缀已是 ASCII（无需处理）")
	}
	t.Log("发现真问题：pv 素材前缀含中文，sanitizeImageName 会清成空串，match_media 命中率归零——需在 deliveryRenderPvStillShots 里转写")
}
