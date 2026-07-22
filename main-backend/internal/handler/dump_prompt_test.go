package handler

import (
	"encoding/json"
	"os"
	"testing"
)

// TestDumpWorkflowPrompt 把真实的系统提示词 + 起手 tools 数组导出成 JSON，
// 供离线探针（scratchpad/probe_ondemand.py）拿去问真实模型「认不认按需加载这套」。
// 只在设置了 DUMP_PROMPT_TO 时运行，平时跳过。
func TestDumpWorkflowPrompt(t *testing.T) {
	dst := os.Getenv("DUMP_PROMPT_TO")
	if dst == "" {
		t.Skip("未设置 DUMP_PROMPT_TO，跳过导出")
	}

	// 走真实装配路径，探针问到的就是线上那份提示词
	p := newWorkflowContextProvider()
	systemPrompt := p.SystemPrompt()

	// mcp_tools 是「全部激活」时的完整工具表，探针拿它模拟 load_tools 的返回
	allMCP := map[string]bool{}
	for _, d := range loadMCPToolDefs() {
		allMCP[d.Function.Name] = true
	}
	payload := map[string]any{
		"system":    systemPrompt,
		"tools":     p.Tools(),
		"mcp_tools": buildCodeWorkflowTools(allMCP),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("已导出到 %s（system %d 字符，常驻工具 %d 个）",
		dst, len(systemPrompt), len(payload["tools"].([]map[string]any)))
}
