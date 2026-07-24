package handler

// chrome_devtools MCP server 接入后的审批分级测试：能操作真实页面/上传本地文件的
// 工具要在 Ask 模式下被拦，纯只读检查类不该被拦；upload_file 的 filePath 参数要
// 被 toolOutsideRoot 正确识别（跟 mcp__fs__ 系工具共用同一套越界判定）。

import (
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/ai/core"
)

func TestChromeDevtoolsDangerousTools(t *testing.T) {
	dangerous := []string{
		"mcp__chrome_devtools__click",
		"mcp__chrome_devtools__fill",
		"mcp__chrome_devtools__fill_form",
		"mcp__chrome_devtools__type_text",
		"mcp__chrome_devtools__drag",
		"mcp__chrome_devtools__press_key",
		"mcp__chrome_devtools__navigate_page",
		"mcp__chrome_devtools__upload_file",
		"mcp__chrome_devtools__handle_dialog",
		"mcp__chrome_devtools__evaluate_script",
	}
	for _, name := range dangerous {
		if !isDangerousTool(name) {
			t.Errorf("%s 应该被判定为危险工具（有真实页面副作用），isDangerousTool 却返回 false", name)
		}
	}

	// 只读检查类：任何模式都该直过，不该拦
	readonly := []string{
		"mcp__chrome_devtools__list_pages",
		"mcp__chrome_devtools__take_screenshot",
		"mcp__chrome_devtools__take_snapshot",
		"mcp__chrome_devtools__list_console_messages",
		"mcp__chrome_devtools__get_console_message",
		"mcp__chrome_devtools__list_network_requests",
		"mcp__chrome_devtools__get_network_request",
		"mcp__chrome_devtools__performance_start_trace",
		"mcp__chrome_devtools__performance_stop_trace",
		"mcp__chrome_devtools__performance_analyze_insight",
		"mcp__chrome_devtools__lighthouse_audit",
		"mcp__chrome_devtools__wait_for",
		"mcp__chrome_devtools__resize_page",
		"mcp__chrome_devtools__select_page",
		"mcp__chrome_devtools__new_page",
		"mcp__chrome_devtools__close_page",
		"mcp__chrome_devtools__hover",
		"mcp__chrome_devtools__emulate",
		"mcp__chrome_devtools__take_heapsnapshot",
	}
	for _, name := range readonly {
		if isDangerousTool(name) {
			t.Errorf("%s 是只读检查类工具，不该被判定为危险，isDangerousTool 却返回 true", name)
		}
	}
}

// upload_file 的本地文件参数叫 filePath（驼峰），跟其余 MCP server 的 snake_case
// path/source/destination/file_path 不一样——不认出这个键名，越界读一个敏感文件
// （比如 main-backend/.env）喂给任意网页时，越界检测和目录级 remember 都会失效。
func TestUploadFileFilePathDetectedAsPathArg(t *testing.T) {
	root := filepath.Clean(core.GetProjectRoot())
	outside := filepath.Join(filepath.Dir(root), "elsewhere", "secret.env")
	outsideJSON := strings.ReplaceAll(outside, `\`, `\\`)

	args := `{"uid":"42","filePath":"` + outsideJSON + `"}`
	got, hit := toolOutsideRoot(args)
	if !got {
		t.Fatalf("upload_file 的 filePath 指向工作目录外时应判定越界，实际未判定: %s", args)
	}
	if hit != outside {
		t.Errorf("越界命中路径不对: got %q, want %q", hit, outside)
	}
}

func TestUploadFileFilePathInsideRootNotFlagged(t *testing.T) {
	args := `{"uid":"42","filePath":"main-backend/foo.txt"}`
	got, _ := toolOutsideRoot(args)
	if got {
		t.Errorf("filePath 在工作目录内不该被判定越界: %s", args)
	}
}
