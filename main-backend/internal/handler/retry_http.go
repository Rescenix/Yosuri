package handler

// retry_http.go —— 模型调用自动重试（2026-08-27 用户要求加入）。
//
// 背景：路由哲学一直是「失败秒切下一个源」。链上有别的源时这很好，
// 但用户精确选中的单个模型（exactModel）没有下一个源可切——一撞上
// 429 限流 / 上游 5xx / 连接 EOF / 超时，整个工作流直接断链，长任务
// 白跑。这里在「秒切」之前，先给当前源做几次带退避的自动重试，
// 把单源抽风吸收掉：撞车就等一会儿再打一次，连续撞满才轮到 failover。
//
// 只重试「暂时性」故障：HTTP 429（限流）、5xx（服务端故障）、连接错误
// （EOF/超时/reset）、流式中途断开。401/403/404（鉴权/额度/下架）与
// 400（请求格式 bug）是确定性失败，重试只会浪费时间，直接交给上层判断。

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxTransientRetries —— 同一源上暂时性故障的自动重试次数（总尝试 = 次数 + 1）。
// 3 次约等于多花 800ms + 1.6s + 1.6s 退避（429 尊重 Retry-After，封顶 8s），
// 不会把工作流拖死；3 次都没好，说明这源这会儿真不行，交给 failover。
const maxTransientRetries = 3

// transientStatus 该状态码是否值得自动重试：限流与服务端故障是暂时性的。
func transientStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// retryWait 两次尝试之间的等待时长。attempt 从 0 开始计。
//   - 429 且响应带 Retry-After（秒）：尊重上游限流时长，但封顶 8s，
//     避免 failover 被拖死（免费档上游 Retry-After 经常给到 60s+）。
//   - 其余暂时性故障：指数退避 800ms / 1.6s。
func retryWait(status int, retryAfter string, attempt int) time.Duration {
	if status == http.StatusTooManyRequests && retryAfter != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 {
			if secs > 8 {
				secs = 8
			}
			return time.Duration(secs) * time.Second
		}
	}
	if attempt == 0 {
		return 800 * time.Millisecond
	}
	return 1600 * time.Millisecond
}

// waitRetry 阻塞等待下一次重试；ctx 被取消（浏览器断开/超时）时立即返回 false，
// 调用方应就此放弃重试。
func waitRetry(ctxDone <-chan struct{}, d time.Duration) bool {
	select {
	case <-ctxDone:
		return false
	case <-time.After(d):
		return true
	}
}
