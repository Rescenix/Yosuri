package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSpaFallbackMiddleware 验证未知前端路由的兜底行为：
// 文档导航 302 回 "/"，静态资源 / fetch / API / 非 GET / 根路径一律原样透传。
func TestSpaFallbackMiddleware(t *testing.T) {
	cases := []struct {
		name         string
		method       string
		path         string
		accept       string
		wantFallback bool
	}{
		{"前端路由 /social 整页跳转", http.MethodGet, "/social", "text/html,application/xhtml+xml", true},
		{"前端路由 /game 整页跳转", http.MethodGet, "/game", "text/html", true},
		{"前端路由 /company", http.MethodGet, "/company", "text/html,*/*", true},
		{"根路径首次加载", http.MethodGet, "/", "text/html", false},
		{"index.html 直接请求", http.MethodGet, "/index.html", "text/html", false},
		{"带斜杠的路径交给 AssetServer", http.MethodGet, "/social/", "text/html", false},
		{"静态资源 404 必须原样返回", http.MethodGet, "/assets/index-old.js", "text/html", false},
		{"图片资源", http.MethodGet, "/favicon.png", "text/html", false},
		{"前端 fetch 拉社交接口", http.MethodGet, "/api/social/friends", "*/*", false},
		{"XHR 不带 text/html", http.MethodGet, "/api/social/friends", "application/json", false},
		{"POST 不受影响", http.MethodPost, "/social", "text/html", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			rec := httptest.NewRecorder()
			spaFallbackMiddleware(next).ServeHTTP(rec, req)

			if tc.wantFallback {
				if nextCalled {
					t.Fatalf("期望走兜底重定向，但请求被透传给了 AssetServer")
				}
				if rec.Code != http.StatusFound {
					t.Fatalf("期望 302，实际 %d", rec.Code)
				}
				if loc := rec.Header().Get("Location"); loc != "/" {
					t.Fatalf("期望 Location=/，实际 %q", loc)
				}
				return
			}

			if !nextCalled {
				t.Fatalf("期望透传给 AssetServer，实际被兜底拦掉了（%d）", rec.Code)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("期望 200，实际 %d", rec.Code)
			}
		})
	}
}
