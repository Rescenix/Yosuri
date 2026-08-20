package handler

// quota_tracker.go —— 免费 API Key 额度追踪（2026-08-18）。
//
// 硅基流动 / 商汤 / 魔搭等多家赠送的免费 API Key
// 各有限额（tokens/调用次数/有效期）。本模块：
//   - 记录每个 backend 的额度查询结果（消耗量、剩余量、刷新时间）
//   - 429 insufficient_quota / Allocated quota exceeded 时标记「额度耗尽」
//   - Auto 链调度：优先排额度未耗尽的 backend，耗尽的沉底（熔断期跳过）
//
// 额度查询接口统一走 OpenAI 兼容的余额敏感错误文本匹配 + 可选用量接口：
//   商汤 / 魔搭：429 insufficient_quota 判定
// 由于各家的免费额度大多按 5 小时/天/月刷新，耗尽标记带刷新时间，
// 到期自动恢复（不用手动干预）。

import (
	"strings"
	"sync"
	"time"
)

// quotaState 单个 backend 的额度状态。
type quotaState struct {
	exhausted   bool      // 是否额度耗尽
	exhaustedAt time.Time // 耗尽时刻
	resetAt     time.Time // 额度刷新时间（到期自动恢复）
	lastQuery   time.Time // 上次查询时刻
	remaining   int64     // 剩余额度（0=未知）
	// usageText 最近一次额度查询的原始文本（诊断用，不参与调度）
	usageText string
}

var (
	quotaMu     sync.Mutex
	quotaStates = map[string]*quotaState{}
)

// quotaKey 与 probeStates 同键：endpoint|model（不同 vendor 天然隔离）。
func quotaKey(b RouterBackend) string { return circuitKey(b) }

// quotaExhausted 检查 backend 是否额度耗尽（且未到刷新时间）。
func quotaExhausted(b RouterBackend) bool {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	st, ok := quotaStates[quotaKey(b)]
	if !ok || !st.exhausted {
		return false
	}
	if !st.resetAt.IsZero() && time.Now().After(st.resetAt) {
		// 刷新时间到，自动恢复
		st.exhausted = false
		return false
	}
	return true
}

// quotaExhaustedDefault 带默认刷新窗口的检查（默认 5 小时，商汤/魔搭免费档惯例）。
func quotaExhaustedDefault(b RouterBackend, defaultReset time.Duration) bool {
	if quotaExhausted(b) {
		return true
	}
	quotaMu.Lock()
	defer quotaMu.Unlock()
	st, ok := quotaStates[quotaKey(b)]
	if !ok || !st.exhausted {
		return false
	}
	if st.resetAt.IsZero() {
		st.resetAt = st.exhaustedAt.Add(defaultReset)
	}
	return time.Now().Before(st.resetAt)
}

// markQuotaExhausted 标记 backend 额度耗尽，resetIn 后自动恢复。
func markQuotaExhausted(b RouterBackend, resetIn time.Duration, reason string) {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	now := time.Now()
	k := quotaKey(b)
	st, ok := quotaStates[k]
	if !ok {
		st = &quotaState{}
		quotaStates[k] = st
	}
	st.exhausted = true
	st.exhaustedAt = now
	st.resetAt = now.Add(resetIn)
	st.lastQuery = now
	st.usageText = reason
}

// markQuotaAvailable 清除额度耗尽标记（真实请求 200 = 额度还在）。
func markQuotaAvailable(b RouterBackend) {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	st, ok := quotaStates[quotaKey(b)]
	if !ok {
		return
	}
	st.exhausted = false
	st.exhaustedAt = time.Time{}
	st.resetAt = time.Time{}
}

// isQuotaExhaustionError 判断错误文本是否「额度耗尽」类错误。
// 识别各家免费档的 429/403 错误体特征：
//   - OpenAI 系（商汤/魔搭/硅基）：
//     "insufficient_quota" / "Allocated quota exceeded" / "quota exceeded"
//   - 通用: "out of quota" / "free quota" / "Monthly usage limit"
func isQuotaExhaustionError(errText string) bool {
	lower := strings.ToLower(errText)
	patterns := []string{
		"insufficient_quota",
		"allocated quota exceeded",
		"quota exceeded",
		"quota_limit",
		"insufficientbalance",
		"exceededquota",
		"out of quota",
		"arrearage",
		"free quota",
		"usage limit",
		"no free quota",
		"no available quota",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// quotaResetWindow 各家免费档默认刷新周期（查不到刷新时间时的兜底）。
var quotaResetWindow = map[string]time.Duration{
	// 商汤：5 小时刷新（官方文档）
	"sensenova": 5 * time.Hour,
	// 魔搭：每日刷新
	"modelscope": 24 * time.Hour,
	// 硅基流动：按账户（长期赠送额度）
	"siliconflow": 24 * time.Hour,
	// Zen 免key：按小时限流（200次/h）
	"zen": 1 * time.Hour,
	// 智谱：永久免费（30 并发），耗尽多为临时限流
	"zhipu": 30 * time.Minute,
}

// defaultQuotaReset 按 backend 的 vendor 特征取默认刷新窗口。
func defaultQuotaReset(b RouterBackend) time.Duration {
	v := strings.ToLower(b.ID + " " + b.BaseURL)
	for k, d := range quotaResetWindow {
		if strings.Contains(v, k) {
			return d
		}
	}
	// 通用默认 5 小时（与商汤同档，避免误判恢复过慢）
	return 5 * time.Hour
}

// markQuotaExhaustedIfError 统一入口：请求失败时按错误文本判定额度耗尽
// 并标记（带默认刷新窗口）。返回是否判定为额度耗尽。
func markQuotaExhaustedIfError(b RouterBackend, errText string) bool {
	if !isQuotaExhaustionError(errText) {
		return false
	}
	markQuotaExhausted(b, defaultQuotaReset(b), truncateChars(errText, 300))
	return true
}

// quotaDebugStatus 生成额度状态摘要（诊断日志/健康接口用）。
func quotaDebugStatus() string {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	var sb strings.Builder
	for k, st := range quotaStates {
		if !st.exhausted {
			continue
		}
		sb.WriteString(k)
		sb.WriteString(": 耗尽@")
		sb.WriteString(st.exhaustedAt.Format("15:04"))
		if !st.resetAt.IsZero() {
			sb.WriteString(" 恢复@")
			sb.WriteString(st.resetAt.Format("15:04"))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}