package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backend/internal/ai/core"
)

// withTempProjectRoot 把项目根指到临时目录并在用例结束后还原。
// SHANXI_WORKDIR_STATE_FILE 也要一起改，否则 SetProjectRoot 会把临时路径
// 写进用户真实的 ~/shanxi_data/workdir.txt，跑完测试工作目录就被换掉了。
func withTempProjectRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SHANXI_WORKDIR_STATE_FILE", filepath.Join(dir, "workdir.txt"))
	old := core.GetProjectRoot()
	if err := core.SetProjectRoot(dir); err != nil {
		t.Fatalf("切临时项目根失败: %v", err)
	}
	t.Cleanup(func() { core.SetProjectRoot(old) })
	return dir
}

func TestCompactToolOutputShortPassthrough(t *testing.T) {
	short := "print('我的世界')"
	if got, arch := compactToolOutput("wf", "call_1", "mcp__shell__run", short); got != short || arch != nil {
		t.Errorf("预算内的输出不该被动，得到 %q", got)
	}
}

func TestCompactToolOutputKeepsHeadAndTail(t *testing.T) {
	withTempProjectRoot(t)

	// 造一个超预算输出：开头是命令，结尾是结论——两头都必须活下来
	head := "$ go test ./...\n"
	body := strings.Repeat("ok  	backend/internal/whatever	0.01s\n", 500)
	tail := "\nFAIL	backend/internal/handler	3 failed\nexit status 1"
	out, _ := compactToolOutput("wf_1", "call_9", "mcp__shell__run", head+body+tail)

	if len(out) >= len(head+body+tail) {
		t.Fatal("超预算输出没有被压缩")
	}
	if !strings.Contains(out, "go test ./...") {
		t.Error("头部丢了——模型将不知道执行的是什么命令")
	}
	// 尾部才是结论所在，老的一刀切截断正是把这里砍掉了
	if !strings.Contains(out, "3 failed") || !strings.Contains(out, "exit status 1") {
		t.Error("尾部结论丢了——这正是分层截断要解决的问题")
	}
	if !strings.Contains(out, "省略") {
		t.Error("没有告诉模型中间被省略了")
	}
}

func TestCompactToolOutputSpillsToReadableFile(t *testing.T) {
	root := withTempProjectRoot(t)

	full := strings.Repeat("行内容\n", 4000)
	out, arch := compactToolOutput("wf_2", "call_1", "mcp__fs__read_text_file", full)

	// 落盘必须在项目根之内，否则 MCP filesystem 的 allowed root 之外，模型读不回来
	spill := filepath.Join(root, ".aurora", "tool_outputs")
	entries, err := os.ReadDir(spill)
	if err != nil || len(entries) != 1 {
		t.Fatalf("完整输出没落到项目根下的 %s: err=%v", spill, err)
	}
	saved, _ := os.ReadFile(filepath.Join(spill, entries[0].Name()))
	if string(saved) != full {
		t.Error("落盘内容与原始输出不一致——截断就不再是无损的了")
	}
	if !strings.Contains(out, ".aurora/tool_outputs") {
		t.Errorf("没把可读回的路径告诉模型: %s", out)
	}

	// 归档记录要能交给账本登记，否则 harness_status 报不出"这条丢了什么"
	if arch == nil {
		t.Fatal("超预算输出没有返回归档记录")
	}
	if arch.Tool != "mcp__fs__read_text_file" || arch.CallID != "call_1" || arch.OmittedChars <= 0 {
		t.Errorf("归档记录内容不对: %+v", arch)
	}

	// 行为已变更：任务成功收尾不再删全文。
	// 原来一成功就删，可上下文里那句「完整输出已存到 xxx」还留在历史里，
	// 指针当场变悬空，下一轮照着去读只会拿到"文件不存在"。现在改由 TTL 淘汰。
	sweepToolOutputArchive()
	if entries, _ := os.ReadDir(spill); len(entries) != 1 {
		t.Error("刚写入的归档不该被 TTL 扫掉（未过期）")
	}

	// 把修改时间推到 TTL 之外，再扫就该清了
	stale := filepath.Join(spill, entries[0].Name())
	old := time.Now().Add(-toolOutputArchiveTTL - time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("改文件时间失败: %v", err)
	}
	sweepToolOutputArchive()
	if entries, _ := os.ReadDir(spill); len(entries) != 0 {
		t.Error("过期归档没有被清掉")
	}
}

func TestCompactToolOutputNoBrokenUTF8(t *testing.T) {
	withTempProjectRoot(t)
	// 全中文（每字 3 字节）最容易在字节切断处切碎
	out, _ := compactToolOutput("wf_3", "call_1", "mcp__shell__run", strings.Repeat("汉", 8000))
	for _, r := range out {
		if r == '�' {
			t.Fatal("截断切碎了多字节字符")
		}
	}
}
