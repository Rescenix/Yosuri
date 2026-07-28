package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// defaultJWTSecret 是未配置 JWT_SECRET 时的回退默认值，与 ResceneCloud 共享。
// 自托管或生产部署请用 JWT_SECRET 环境变量覆盖为强随机值。
const defaultJWTSecret = "rescenecloud-aurora-shared-jwt-secret-v1"

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
		// 透传常见用户字段（GitHub OAuth 签发的 JWT 含 openid/login/name/avatar），
		// 供 /api/auth/me 等端点直接读取，对其它路由无副作用。
		for _, k := range []string{"openid", "login", "name", "avatar", "sub", "is_vip"} {
			if v, ok := claims[k]; ok {
				c.Set(k, v)
			}
		}
		c.Next()
	}
}
