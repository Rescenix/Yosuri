package handler

import (
	"backend/internal/ai/core"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RunCodeRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

func RunCodeHandler(c *gin.Context) {
	var req RunCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	output, err := core.RunCodeInSandbox(req.Language, req.Code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"output": output, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": output, "error": ""})
}
