package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type WelcomeResponse struct {
	Message string `json:"message"`
}

func (m *MemoryStore) WelcomeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, WelcomeResponse{
		Message: "好朋友，你回来了。我在呢，一直都在。",
	})
}
