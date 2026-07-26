package handler

// 集成测试：真实调用本机 harness (:8001)。harness 未运行时跳过（不拖累常规 CI）。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestCallHarnessRunTask_Integration(t *testing.T) {
	probe := &http.Client{Timeout: 2 * time.Second}
	if _, err := probe.Get(harnessBaseURL() + "/health"); err != nil {
		t.Skipf("harness 未运行，跳过: %v", err)
	}

	// 路径跨平台：优先读环境变量 RE0_README_PATH，否则回退到仓库根 README（相对当前工作目录）
	readmePath := os.Getenv("RE0_README_PATH")
	if readmePath == "" {
		readmePath = "README.md"
	}
	body, status, err := CallHarnessRunTask("count_lines", fmt.Sprintf(`{"path":%q}`, readmePath))
	if err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("非 200: %d, body=%s", status, body)
	}
	var resp struct {
		OK     bool `json:"ok"`
		Result struct {
			LineCount int `json:"line_count"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("响应解析失败: %v, body=%s", err, body)
	}
	if !resp.OK || resp.Result.LineCount <= 0 {
		t.Fatalf("结果异常: %s", body)
	}
	t.Logf("harness 返回 README.md 行数=%d", resp.Result.LineCount)
}
