package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTransientStatus(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{http.StatusOK, false},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{451, false}, // censorship — deterministic per-content, retry pointless
	}
	for _, tc := range tests {
		got := transientStatus(tc.code)
		if got != tc.want {
			t.Errorf("transientStatus(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestRetryWait(t *testing.T) {
	// 429 不带 Retry-After：指数退避
	d0 := retryWait(http.StatusTooManyRequests, "", 0)
	if d0 < 600*time.Millisecond || d0 > 1000*time.Millisecond {
		t.Errorf("429 no Retry-After attempt 0: %v, want ~800ms", d0)
	}
	d1 := retryWait(http.StatusTooManyRequests, "", 1)
	if d1 < 1200*time.Millisecond || d1 > 2000*time.Millisecond {
		t.Errorf("429 no Retry-After attempt 1: %v, want ~1600ms", d1)
	}

	// 429 带 Retry-After：尊重
	d2 := retryWait(http.StatusTooManyRequests, "3", 0)
	if d2 < 2*time.Second || d2 > 4*time.Second {
		t.Errorf("429 Retry-After=3: %v, want ~3s", d2)
	}

	// 429 Retry-After 封顶 8s
	d3 := retryWait(http.StatusTooManyRequests, "30", 0)
	if d3 > 9*time.Second {
		t.Errorf("429 Retry-After=30 should cap at 8s, got %v", d3)
	}
	if d3 < 7*time.Second {
		t.Errorf("429 Retry-After=30 should cap at 8s, got %v (too short)", d3)
	}

	// 5xx：指数退避
	d4 := retryWait(500, "", 0)
	if d4 < 600*time.Millisecond || d4 > 1000*time.Millisecond {
		t.Errorf("5xx attempt 0: %v, want ~800ms", d4)
	}
	d5 := retryWait(503, "", 1)
	if d5 < 1200*time.Millisecond || d5 > 2000*time.Millisecond {
		t.Errorf("5xx attempt 1: %v, want ~1600ms", d5)
	}
}

func TestWaitRetryCancel(t *testing.T) {
	done := make(chan struct{})
	close(done) // already cancelled
	got := waitRetry(done, 10*time.Minute)
	if got {
		t.Error("waitRetry on cancelled channel should return false")
	}
}

func TestWaitRetryNormal(t *testing.T) {
	done := make(chan struct{})
	start := time.Now()
	got := waitRetry(done, 50*time.Millisecond)
	elapsed := time.Since(start)
	if !got {
		t.Error("waitRetry on normal channel should return true")
	}
	if elapsed < 30*time.Millisecond {
		t.Error("waitRetry returned too fast")
	}
}

// ---------- 真实 HTTP 集成测试：重试循环确实会重试 ----------

func TestOpenAIChatOnceRetries429ThenSucceeds(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			// 第一次 429（带 Retry-After=0，走指数退避），第二次直接 200
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"hi after retry"}}]}`))
	}))
	defer srv.Close()

	b := RouterBackend{ID: "test-free-1", Name: "test", BaseURL: srv.URL, Model: "m", Timeout: 5 * time.Second, Source: "custom"}
	content, calls, err := openAIChatOnce(context.Background(), b, []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err != nil {
		t.Fatalf("openAIChatOnce returned error: %v", err)
	}
	if content != "hi after retry" {
		t.Errorf("content = %q, want %q", content, "hi after retry")
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Errorf("attempts = %d, want 2 (1x429 + 1x200)", got)
	}
}

func TestOpenAIChatOnceGivesUpAfterRetriesExhausted(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":{"message":"boom"}}`))
	}))
	defer srv.Close()

	b := RouterBackend{ID: "test-free-2", Name: "test", BaseURL: srv.URL, Model: "m", Timeout: 5 * time.Second, Source: "custom"}
	_, _, err := openAIChatOnce(context.Background(), b, []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("error = %v, want HTTP 503 mention", err)
	}
	if got := atomic.LoadInt32(&attempts); got != int32(maxTransientRetries+1) {
		t.Errorf("attempts = %d, want %d", got, maxTransientRetries+1)
	}
}

func TestOpenAIChatOnceNoRetryOn401(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	b := RouterBackend{ID: "test-free-3", Name: "test", BaseURL: srv.URL, Model: "m", Timeout: 5 * time.Second, Source: "custom"}
	_, _, err := openAIChatOnce(context.Background(), b, []map[string]any{{"role": "user", "content": "hi"}}, nil)
	if err == nil {
		t.Fatal("expected 401 error")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("attempts = %d, want 1 (deterministic 401 must NOT retry)", got)
	}
}