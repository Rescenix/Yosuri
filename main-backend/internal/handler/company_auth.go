package handler

// company_auth.go — 公司 API 鉴权中间件
//
// 2026-08-26：评分/花钱/改标签等写操作必须带 API Key（Authorization: Bearer <key>）。
// key 从环境变量 COMPANY_API_KEY 读取；未设置 = 本地开发模式放行（兼容桌面版 wails 后端）。
// 云端部署必须设置 COMPANY_API_KEY，否则任何人可刷评分/花公司钱。

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// companyAuthRequired 校验 Authorization: Bearer <COMPANY_API_KEY>
// 未配置 key → 放行（本地开发模式）；配置了 → 必须匹配
func companyAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(os.Getenv("COMPANY_API_KEY"))
		if key == "" {
			c.Next() // 本地开发模式：没设 key 不拦
			return
		}
		auth := c.GetHeader("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token != key {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权：需要有效的 API Key"})
			return
		}
		c.Next()
	}
}
