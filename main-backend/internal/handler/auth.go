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

	if adminPassword == "" || jwtSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器配置错误，请联系管理员"})
		return
	}

	// 开发模式后门：如果环境变量 DEV_MODE=true，任何密码都通过，并签发 token
	devMode := os.Getenv("DEV_MODE") == "true"
	if !devMode && req.Password != adminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role": "admin",
		"exp":  time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(jwtSecret))

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}
