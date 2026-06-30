package handler

import (
	"backend/internal/ai/core"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleExecuteMarker(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	marker := string(body)

	toolName, args, err := core.ExtractToolArgs(marker)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	argsJSON, _ := json.Marshal(args)
	toolCall := core.ToolCall{
		ID:   "ds_browser_tool_" + toolName,
		Type: "function",
		Function: core.ToolCallFunc{
			Name:      toolName,
			Arguments: string(argsJSON),
		},
	}

	result, err := core.ExecuteToolCall(toolCall)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	if result.Failed {
		c.String(http.StatusOK, "工具执行失败: "+result.Content)
	} else {
		c.String(http.StatusOK, result.Content)
	}
}
