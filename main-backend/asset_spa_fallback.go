package main

import (
	"net/http"
	"strings"
)

// spaFallbackMiddleware 把「AssetServer 里没有对应文件的前端路由」拉回应用入口。
//
// 背景：桌面版页面源是 https://wails.localhost，Wails 的 AssetServer 只服务 embed 进来的
// 静态文件，没有 /social、/game 这类 vue-router 路由对应的文件。前端正常跳转走的是
// history.pushState（不发文档请求），但只要有代码用 window.location.href 或 <a href>
// 做整页跳转，WebView2 就会真的去请求那个路径，AssetServer 找不到文件 → 返回 404 →
// 窗口被浏览器风格的「找不到此 wails.localhost 页」顶掉，整个应用不可用。
//
// 这里做兜底：GET 的文档导航（Accept 含 text/html）请求一个不带扩展名、也不在 assets 里的
// 路径时，302 回 "/" 让应用重新加载，而不是把用户丢在 404 死页上。
//
// 影响面严格限定在「本来就会 404 的请求」：静态资源请求（路径带扩展名，404 是真错，
// 需要原样返回给前端）和所有 fetch/XHR/API 请求（Accept 不含 text/html）都不受影响。
func spaFallbackMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if shouldFallbackToApp(req) {
			rw.Header().Set("Cache-Control", "no-store")
			http.Redirect(rw, req, "/", http.StatusFound)
			return
		}
		next.ServeHTTP(rw, req)
	})
}

func shouldFallbackToApp(req *http.Request) bool {
	if req.Method != http.MethodGet {
		return false
	}
	// 只认顶层文档导航：WebView2 跳转会带 Accept: text/html，fetch/XHR 不会。
	if !strings.Contains(req.Header.Get("Accept"), "text/html") {
		return false
	}
	path := req.URL.Path
	if path == "" || path == "/" || path == "/index.html" {
		return false // AssetServer 自己会喂 index.html
	}
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, "/index.html") {
		return false
	}
	// 最后一段带扩展名 = 静态资源请求，404 就是真 404，不能拿首页顶替。
	last := path[strings.LastIndex(path, "/")+1:]
	if strings.Contains(last, ".") {
		return false
	}
	return true
}
