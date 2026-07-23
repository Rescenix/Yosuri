package handler

// MCP filesystem server 越界读的集成测试：真的把 npx server-filesystem 拉起来，
// 通过 Go 客户端读一个工作目录之外的文件。
//
// 这条链路是本次改动的关键——审批闸门放行了，底层也必须真能执行；以前这里会
// 返回 "path outside allowed directories"。需要联网/已缓存的 npx，设
// SKIP_MCP_INTEGRATION=1 可跳过。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/ai/core"
)

func TestMCPFilesystemReadsOutsideProjectRoot(t *testing.T) {
	if os.Getenv("SKIP_MCP_INTEGRATION") != "" {
		t.Skip("SKIP_MCP_INTEGRATION 已设置")
	}

	root := filepath.Clean(core.GetProjectRoot())

	// 在系统临时目录（必定在工作目录之外）造一个文件
	tmpFile := filepath.Join(t.TempDir(), "outside_probe.txt")
	const want = "hello from outside the workdir"
	if err := os.WriteFile(tmpFile, []byte(want), 0o644); err != nil {
		t.Fatalf("准备测试文件失败: %v", err)
	}
	if !pathOutsideRoot(tmpFile) {
		t.Fatalf("测试前提不成立：%q 应该在工作目录 %q 之外", tmpFile, root)
	}

	// 只拉 fs 一个 server，避免把其余 server 也启起来拖慢测试
	cfgPath := filepath.Join(t.TempDir(), "mcp.json")
	cfg := `{"servers":{"fs":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","` +
		strings.ReplaceAll(root, `\`, `\\`) + `"]}}}`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("写 mcp.json 失败: %v", err)
	}
	t.Setenv("MCP_CONFIG", cfgPath)

	ReinitMCP()
	defer ReinitMCP() // 还原成仓库真实配置，别污染后续测试

	if len(loadMCPToolDefs()) == 0 {
		t.Skip("MCP fs server 未能启动（npx 不可用？），跳过")
	}

	args := `{"path":"` + strings.ReplaceAll(tmpFile, `\`, `\\`) + `"}`
	out, err := callMCPTool("mcp__fs__read_text_file", args)
	if err != nil {
		t.Fatalf("读工作目录之外的文件失败: %v", err)
	}
	if strings.Contains(out, "outside allowed directories") {
		t.Fatalf("底层仍在拦越界访问，审批放行也没用: %s", out)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("读到的内容不对:\n got=%q\nwant 包含=%q", out, want)
	}
}
