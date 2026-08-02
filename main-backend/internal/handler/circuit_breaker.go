package handler

// circuit_breaker.go —— Auto 智能路由的免费池熔断器。
//
// 语义：免费池条目连续失败 N 次（网络错误 / 429 / 5xx）→ 熔断打开，
// 冷却期内 Auto 路由（resolveBackends 全链）跳过该条目，秒切下一个可用源；
// 冷却结束自动恢复（半开探测：恢复后下一次失败会重新计数）。
// 成功一次即清零失败计数——能成功说明上游已恢复。
//
// 为什么不做永久禁用：401/403 是确定性不可用，已有 disableFreeModel 永久标记；
// 429 限流 / 5xx / 超时是暂时性故障，永久禁用会一棍子打死可能很快就恢复的免费档。
// 熔断只作用于 Auto 全链路由；用户在设置面板精确选中的模型不受影响
// （resolveExact 不走熔断过滤，用户手选就要试）。

import (
	"fmt"
	"sync"
	"time"
)

// circuitState 单个免费模型条目的熔断状态。
type circuitState struct {
	failures int       // 连续失败次数（成功清零）
	openedAt time.Time // 熔断打开时刻（零值 = 未熔断）
}

var (
	circuitMu      sync.Mutex
	circuitStates  = map[string]*circuitState{}
)

const (
	// circuitThreshold 连续失败多少次触发熔断（含）。免费档 429 很常见，3 次才开闸。
	circuitThreshold = 3
	// circuitCooldown 熔断打开后冷却多久自动恢复。
	circuitCooldown = 60 * time.Second
)

// circuitKey 按 backend 的唯一标识（BaseURL|Model）追踪熔断状态。
func circuitKey(b RouterBackend) string {
	return b.BaseURL + "|" + b.Model
}

// circuitOpen 判断该 backend 当前是否处于熔断冷却期。
// 只对免费池条目生效（Source=="free"）；用户自定义提供方不熔断。
func circuitOpen(b RouterBackend) bool {
	if b.Source != "free" {
		return false
	}
	circuitMu.Lock()
	defer circuitMu.Unlock()
	st, ok := circuitStates[circuitKey(b)]
	if !ok || st.failures < circuitThreshold {
		return false
	}
	if time.Since(st.openedAt) > circuitCooldown {
		// 冷却结束：自动恢复（清零），下次请求重新探测
		st.failures = 0
		st.openedAt = time.Time{}
		return false
	}
	return true
}

// circuitFail 记录一次失败。达到阈值时打开熔断。
func circuitFail(b RouterBackend) {
	if b.Source != "free" {
		return
	}
	circuitMu.Lock()
	defer circuitMu.Unlock()
	st := circuitStates[circuitKey(b)]
	if st == nil {
		st = &circuitState{}
		circuitStates[circuitKey(b)] = st
	}
	st.failures++
	if st.failures == circuitThreshold {
		st.openedAt = time.Now()
		fmt.Printf("🔴 [熔断] %s 连续失败 %d 次，熔断 %ds，Auto 路由暂时跳过\n",
			b.Name, circuitThreshold, int(circuitCooldown/time.Second))
	}
}

// circuitSuccess 记录一次成功，清零失败计数（上游已恢复）。
func circuitSuccess(b RouterBackend) {
	if b.Source != "free" {
		return
	}
	circuitMu.Lock()
	defer circuitMu.Unlock()
	if st, ok := circuitStates[circuitKey(b)]; ok && st.failures > 0 {
		st.failures = 0
		st.openedAt = time.Time{}
		fmt.Printf("🟢 [熔断] %s 恢复成功，失败计数清零\n", b.Name)
	}
}
