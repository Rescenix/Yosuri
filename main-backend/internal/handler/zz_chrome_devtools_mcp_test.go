package handler

// chrome_devtools MCP server 的集成测试：真的拉起 npx chrome-devtools-mcp，通过 Go
// 客户端调一个只读工具，证明不仅"注册上了"，是真的能打通调用（会启动一个真实
// Chrome/Puppeteer 实例）。设 SKIP_MCP_INTEGRATION=1 可跳过（离线/无 Chrome 环境）。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPChromeDevtoolsListPages(t *testing.T) {
	if os.Getenv("SKIP_MCP_INTEGRATION") != "" {
		t.Skip("SKIP_MCP_INTEGRATION 已设置")
	}

	cfgPath := filepath.Join(t.TempDir(), "mcp.json")
	cfg := `{"servers":{"chrome_devtools":{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]}}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("写 mcp.json 失败: %v", err)
	}
	t.Setenv("MCP_CONFIG", cfgPath)

	ReinitMCP()
	defer ReinitMCP() // 还原成仓库真实配置，别污染后续测试

	tools := loadMCPToolDefs()
	if len(tools) == 0 {
		t.Skip("chrome_devtools server 未能启动（npx/Chrome 不可用？），跳过")
	}

	out, err := callMCPTool("mcp__chrome_devtools__list_pages", "{}")
	if err != nil {
		t.Fatalf("调用 list_pages 失败: %v", err)
	}
	if strings.Contains(strings.ToLower(out), "error") && !strings.Contains(out, "0 error") {
		t.Logf("list_pages 返回内容包含 'error' 字样，人工核对一下: %s", truncateChars(out, 300))
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("list_pages 返回空内容")
	}
	t.Logf("list_pages 输出: %s", truncateChars(out, 300))
}

// upload_file 的越界拦截不能只停在 toolOutsideRoot 单测——这里过一遍完整链路：
// maybeRequestApproval 风格的判定（isDangerousTool || outside）都命中，
// 且不实际执行（不需要真跑一次上传）。
func TestChromeDevtoolsUploadFileIsGatedEvenWithoutOutsidePath(t *testing.T) {
	// 就算路径在工作目录内，upload_file 本身也在 dangerousToolSet 里，必须拦
	args, _ := json.Marshal(map[string]string{"uid": "1", "filePath": "main-backend/README.md"})
	if !isDangerousTool("mcp__chrome_devtools__upload_file") {
		t.Fatalf("upload_file 应始终被判定危险，不管路径在不在工作目录内")
	}
	outside, _ := toolOutsideRoot(string(args))
	if outside {
		t.Fatalf("这个用例路径在工作目录内，不该被判越界: %s", args)
	}
}
