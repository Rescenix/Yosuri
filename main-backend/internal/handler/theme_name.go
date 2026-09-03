package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// generateThemeNameWithAuto 用 auto 免费池为颜色生成 2-4 个汉字的语义主题名
// （像「矢车菊」「樱花」那种）。走 resolveBackends("", "") 全链兜底，与主对话
// auto 口径一致——用户配置的提供方优先，否则免费池。
// 失败返回空串，由调用方回退「自定义」占位，绝不阻塞配色切换。
func generateThemeNameWithAuto(ctx context.Context, colorHex string) string {
	colorHex = strings.TrimSpace(colorHex)
	if colorHex == "" {
		return ""
	}
	backends := resolveBackends("", "")
	if len(backends) == 0 {
		return ""
	}
	prompt := fmt.Sprintf(`你是色彩命名器。根据给定的十六进制颜色，起一个贴合色感的 2-4 个汉字中文名字。
要求：
- 像「矢车菊」「樱花」「薰衣草」「金盏花」这样有画面感、贴合颜色的词
- 不要引号、不要「颜色：」之类前缀、不要英文、不要解释
- 直接输出名字本身

颜色：%s`, colorHex)

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	// 起名是旁路任务：超时 15s 放弃
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	content, _, err := openAIChatOnce(ctx, backends[0], msgs, nil)
	if err != nil {
		return ""
	}
	return cleanTitle(content)
}

// HandleThemeName POST /api/theme/name
// body: { color: "#3b82f6" }
// 返回: { name: 语义主题名，失败时为空串 }
func HandleThemeName(c *gin.Context) {
	var body struct {
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	name := generateThemeNameWithAuto(c.Request.Context(), body.Color)
	c.JSON(200, gin.H{"name": name})
}