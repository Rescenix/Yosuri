package handler

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// isLocalOrigin 判断请求 Origin 是否是本机自己的界面（开发用 vite/浏览器、
// 打包用 Wails 桌面壳）。re0 的 API 能起终端进程、读写工作区文件，属于高危面，
// 绝不能对任意网站开放跨源读写——否则用户访问一个恶意网页，网页里的 JS 就能
// 直接驱动本地后端执行命令（drive-by RCE）。
//
// 放行：http(s)://127.0.0.1:<任意端口>、http(s)://localhost:<任意端口>、
// Wails 桌面壳的 http(s)://wails.localhost 与 wails://app。
// 其余（含 null、file://、任何公网域名）一律不放行。
func isLocalOrigin(origin string) bool {
	if origin == "" {
		return true // 无 Origin：同源导航 / curl / 本机非浏览器调用，浏览器不会发这种跨源请求
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch u.Scheme {
	case "wails":
		return true
	case "http", "https":
	default:
		return false
	}
	if host == "localhost" || host == "wails.localhost" || strings.HasSuffix(host, ".wails.localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

// corsLocalOnly 只对本机界面放行跨源，其他站点拿不到响应。
// 返回 true 表示这是预检请求，调用方应直接结束，不再进业务 handler。
func corsLocalOnly(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if isLocalOrigin(origin) && origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Guest-Uid")
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}
