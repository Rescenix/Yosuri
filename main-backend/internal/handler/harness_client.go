package handler

// Python Harness (:8001) 的 HTTP 客户端 —— Go 后端调用长驻 harness 的集成示例。
//
// harness 是 C:\Pro2026\re0\harness 下的独立常驻运行时（FastAPI + watchdog），
// 承担 MCP 工具生态、日志自检等能力；Go 侧只做一次普通的 http.Get。
//
// 路由: GET /api/harness/demo?tool=list_directory&path=C:\Pro2026\re0

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func harnessBaseURL() string {
	if u := os.Getenv("HARNESS_URL"); u != "" {
		return u
	}
	return "http://localhost:8001"
}

var harnessHTTP = &http.Client{Timeout: 60 * time.Second}

// CallHarnessRunTask 调 harness 的 /run_task。argsJSON 是工具参数的 JSON 字符串。
func CallHarnessRunTask(tool, argsJSON string) ([]byte, int, error) {
	q := url.Values{}
	q.Set("tool", tool)
	q.Set("args", argsJSON)
	resp, err := harnessHTTP.Get(harnessBaseURL() + "/run_task?" + q.Encode())
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// HandleHarnessDemo 集成示例端点：把请求转给 harness 并原样返回结果。
func HandleHarnessDemo(c *gin.Context) {
	tool := c.DefaultQuery("tool", "list_directory")
	argsJSON := c.Query("args")
	if argsJSON == "" {
		if path := c.Query("path"); path != "" {
			raw, _ := json.Marshal(map[string]string{"path": path})
			argsJSON = string(raw)
		} else {
			argsJSON = "{}"
		}
	}
	body, status, err := CallHarnessRunTask(tool, argsJSON)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "harness 不可达: " + err.Error()})
		return
	}
	c.Data(status, "application/json; charset=utf-8", body)
}
