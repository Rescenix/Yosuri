package handler

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type LoginRequest struct {
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	jwtSecret := os.Getenv("JWT_SECRET")

	// 防御1：密码为空时拒绝所有登录
	if adminPassword == "" || jwtSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器配置错误，请联系管理员"})
		return
	}

	// 防御2：密码不匹配时拒绝
	if req.Password != adminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	// 密码匹配后才签发 token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": "admin",
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(jwtSecret))

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}
