package handler

import (
	"strings"
	"testing"
)

// TestDeliverySourcesMarkdown 验证调研报告「参考来源」段落由真实搜索结果 URL 确定性生成，
// 不依赖模型自觉抄链接（Bing 免 key 联网，实测可抓取）。
func TestDeliverySourcesMarkdown(t *testing.T) {
	urls := []string{
		"https://example.com/a",
		"https://example.com/b",
	}
	out := deliverySourcesMarkdown("番茄钟学生冲刺台", urls)
	if !strings.Contains(out, "参考来源") {
		t.Fatalf("缺少参考来源标题: %s", out)
	}
	if !strings.Contains(out, "https://example.com/a") || !strings.Contains(out, "https://example.com/b") {
		t.Fatalf("未包含全部真实 URL: %s", out)
	}
	if !strings.Contains(out, "检索时间") || !strings.Contains(out, "番茄钟学生冲刺台") {
		t.Fatalf("缺少检索时间/检索词: %s", out)
	}
}

// TestDeliverySourcesMarkdownEmpty 无结果时返回空串（不追加来源段）。
func TestDeliverySourcesMarkdownEmpty(t *testing.T) {
	if out := deliverySourcesMarkdown("x", nil); out != "" {
		t.Fatalf("空来源应返回空串，实际: %s", out)
	}
}

// TestValidatePvFallbackGate 验证审批门禁能放行生图兜底清单（kind=pv-fallback-images 的 JSON），
// 而非法 JSON 会被拒绝——确保「生图素材代替视频」不算缺失、也不算假交付。
func TestValidatePvFallbackGate(t *testing.T) {
	good := []byte(`{"kind":"pv-fallback-images","files":["a.png"],"reason":"x"}`)
	if err := validateProjectEvidenceFormat(t.TempDir(), projectDeliveryEvidence{Stage: "pv", File: "06-宣传PV.manifest.json"}, good); err != nil {
		t.Fatalf("合法的生图兜底清单应通过，实际: %v", err)
	}
	bad := []byte(`{"kind":"other"}`)
	if err := validateProjectEvidenceFormat(t.TempDir(), projectDeliveryEvidence{Stage: "pv", File: "06-宣传PV.manifest.json"}, bad); err == nil {
		t.Fatal("不含 pv-fallback-images 标记的 JSON 应被拒绝")
	}
}
