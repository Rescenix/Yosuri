package handler

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed overlay.html
var overlayPageHTML []byte

//go:embed overlay_icon.png
var overlayIconPNG []byte

// HandleOverlayPage GET /overlay —— 桌面悬浮球窗口加载的页面（球 + 展开面板），
// 数据全部走 /api/agent/watch 的 EventSource，不需要任何模板变量。
func HandleOverlayPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", overlayPageHTML)
}

// HandleOverlayIcon GET /overlay/icon.png —— 悬浮球用的应用图标（appicon.png 缩到 256x256）。
func HandleOverlayIcon(c *gin.Context) {
	c.Data(http.StatusOK, "image/png", overlayIconPNG)
}
