package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// defaultJWTSecret 已废弃：re0 开源侧不持有任何鉴权密钥（见 cloud_auth.go 头部约定）。
// /api/auth/me 改为直接代理到 ResceneCloud 云端验签，不再本地验 JWT。
// 此常量仅作为编译占位保留，切勿填入任何真实密钥（开源代码零密钥是安全底线）。
const defaultJWTSecret = ""

// jwtSecret 返回 JWT 签名密钥：优先取环境变量，未配置则回退默认值。
func jwtSecret() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte(defaultJWTSecret)
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证token"})
			c.Abort()
			return
		}

		// 动态读取密钥（env 优先，未配置则回退硬编码默认值，保证开箱即验 token）
		jwtSecret := jwtSecret()

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			// 安全截断 token 预览，避免 token 长度 < 20 时 slice 越界 panic
			preview := tokenString
			if len(preview) > 20 {
				preview = preview[:20] + "..."
			}
			fmt.Printf("❌ JWT验证失败: %v, token(len=%d): %s\n", err, len(tokenString), preview)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效token"})
			c.Abort()
			return
		}

		claims, _ := token.Claims.(jwt.MapClaims)
		c.Set("role", claims["role"])
		// 透传常见用户字段（GitHub OAuth 签发的 JWT 含 openid/login/name/avatar/uid），
		// 供 /api/auth/me 等端点直接读取，对其它路由无副作用。
		for _, k := range []string{"openid", "login", "name", "avatar", "sub", "uid", "is_vip"} {
			if v, ok := claims[k]; ok {
				c.Set(k, v)
			}
		}
		c.Next()
	}
}
