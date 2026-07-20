package handler

import (
	"net/http"

	"backend/internal/agent"

	"github.com/gin-gonic/gin"
)

// WorkflowRunner 工作流编排执行器
type WorkflowRunner struct {
	chatHandler *ChatHandler
}

const estimatedContextWindow = 128000

func estimateTokenCount(s string) int {
	return len(s) / 4
}

func NewWorkflowRunner(chatHandler *ChatHandler) *WorkflowRunner {
	return &WorkflowRunner{chatHandler: chatHandler}
}

// HandleListWorkflows GET /api/workflows — 列出可用工作流
func (r *WorkflowRunner) HandleListWorkflows(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"workflows": agent.ListWorkflows(),
	})
}
