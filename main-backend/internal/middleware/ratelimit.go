package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]time.Time
	interval time.Duration // 限流间隔
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]time.Time),
		interval: 30 * time.Second, // 默认间隔
	}
}
func NewStrictRateLimiter() *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]time.Time),
		interval: 20 * time.Second, // 严格限流
	}
}
func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 如果携带有效 JWT 且角色为 admin，跳过限流
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				return jwtSecret(), nil
			})
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok && claims["role"] == "admin" {
					c.Next()
					return
				}
			}
		}

		// 2. 未登录，按 IP 限流
		rl.mu.Lock()
		defer rl.mu.Unlock()

		ip := c.ClientIP()
		lastVisit, exists := rl.visitors[ip]

		if exists && time.Since(lastVisit) < rl.interval {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "访问过于频繁，请30秒后再试"})
			c.Abort()
			return
		}

		rl.visitors[ip] = time.Now()
		c.Next()
	}
}
