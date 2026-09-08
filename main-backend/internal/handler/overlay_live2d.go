package handler

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// live2dAssets 内嵌看板娘运行时：官方样例模型(Hiyori) + Cubism Web core +
// pixi / pixi-live2d-display。全部随二进制分发，断网也能跑，不依赖任何 CDN。
//
//go:embed all:live2d
var live2dAssets embed.FS

// HandleLive2DAsset GET /overlay/live2d/* —— 从内嵌文件系统里吐模型和运行时。
// 路径清洗后只允许 live2d/ 子树内的文件，杜绝 ../ 穿越。
func HandleLive2DAsset(c *gin.Context) {
	name := strings.TrimPrefix(c.Param("name"), "/")
	name = path.Clean("/" + name)[1:]
	if name == "" || strings.Contains(name, "..") {
		c.String(http.StatusBadRequest, "bad path")
		return
	}
	data, err := fs.ReadFile(live2dAssets, "live2d/"+name)
	if err != nil {
		c.String(http.StatusNotFound, "not found")
		return
	}
	c.Data(http.StatusOK, live2dContentType(name), data)
}

func live2dContentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	default:
		// .moc3 / .physics3.json 等二进制或自定义后缀：按 octet-stream 给，
		// Cubism 运行时自己解析，不看 MIME。
		return "application/octet-stream"
	}
}
