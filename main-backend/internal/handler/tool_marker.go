package handler

import (
	"backend/internal/ai/core"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

func HandleExecuteMarker(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil || len(body) == 0 {
		c.String(http.StatusBadRequest, "缺少请求体")
		return
	}
	marker := strings.TrimSpace(string(body))

	// 解析 [TOOL:toolName arg1="val1" arg2="val2"...] 格式
	re := regexp.MustCompile(`\[TOOL:(\w+)\s+(.*?)\]`)
	matches := re.FindStringSubmatch(marker)
	if len(matches) < 3 {
		c.String(http.StatusBadRequest, "无法解析工具调用标记")
		return
	}

	toolName := matches[1]
	argsStr := strings.TrimSpace(matches[2])

	// 将 key="value" 转换为 JSON 对象字符串
	argPairs := regexp.MustCompile(`(\w+)="(.*?)"`).FindAllStringSubmatch(argsStr, -1)
	args := make(map[string]interface{})
	for _, pair := range argPairs {
		if len(pair) == 3 {
			args[pair[1]] = pair[2]
		}
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
