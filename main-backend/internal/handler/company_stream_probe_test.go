package handler

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestCompanyStreamProbe 真实探测：公司池的健康源是否支持 stream=true。
// 生产大屏的逐字流依赖这一点；不支持的源会走非流式回退（功能不坏，但体验降级）。
// 探测用极小 max_tokens，不烧额度。
func TestCompanyStreamProbe(t *testing.T) {
	if testing.Short() {
		t.Skip("-short 跳过联网探测")
	}
	backends := companyModelBackends()
	if len(backends) == 0 {
		t.Skip("公司模型池无可用源（离线环境）")
	}
	ok, fail := 0, 0
	for _, b := range backends {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		var got strings.Builder
		_, err := chatBackendStream(ctx, b, "只回复两个字：收到", func(d string) { got.WriteString(d) })
		cancel()
		if err != nil {
			fail++
			t.Logf("✗ %-28s 不支持流式/失败: %v", b.Name, err)
		} else {
			ok++
			t.Logf("✓ %-28s 流式可用，首块=%q", b.Name, deliveryTruncate(got.String(), 20))
		}
	}
	t.Logf("流式支持：%d/%d", ok, ok+fail)
	if ok == 0 {
		t.Skip("当前所有源都不支持流式——大屏将全程走整段回退（记录事实，不判失败）")
	}
}
