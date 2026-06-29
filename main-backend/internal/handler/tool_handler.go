// handler/tool_handler.go
package handler

import (
	"os"
	"os/exec"

	"github.com/gin-gonic/gin"
)

func HandleToolExecute(c *gin.Context) {
	var req struct {
		Tool string            `json:"tool"`
		Args map[string]string `json:"args"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(400, "参数错误")
		return
	}

	switch req.Tool {
	case "read_file":
		content, err := os.ReadFile(req.Args["path"])
		if err != nil {
			c.String(500, "读取失败: "+err.Error())
			return
		}
		c.String(200, string(content))
	case "write_file":
		err := os.WriteFile(req.Args["path"], []byte(req.Args["content"]), 0644)
		if err != nil {
			c.String(500, "写入失败: "+err.Error())
			return
		}
		c.String(200, "写入成功")
	case "execute_command":
		cmd := exec.Command("cmd", "/C", req.Args["command"])
		output, err := cmd.CombinedOutput()
		if err != nil {
			c.String(500, "执行失败: "+err.Error()+string(output))
			return
		}
		c.String(200, string(output))
	default:
		c.String(400, "未知工具")
	}
}
