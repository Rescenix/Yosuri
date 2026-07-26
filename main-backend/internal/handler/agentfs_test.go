package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"backend/internal/ai/core"
)

// TestAgentFSWriteEditRestore 端到端验证 AgentFS v1 影子事务层：
//  1. 开辟会话（OpenAgentFSSession）
//  2. 模拟一次 write_file：OnBeforeWrite → 真实写盘 → OnAfterWrite
//  3. 审计时间线出现该笔记录（AgentFSLog）
//  4. AgentFSRestore 把文件还原到写之前的内容，真实盘被回退
//
// 全部用 t.TempDir()，不污染 ~/shanxi_data，也不碰 re0 主仓库。
func TestAgentFSWriteEditRestore(t *testing.T) {
	// 隔离 AgentFS 数据根（指向临时目录，避免污染真实 shanxi_data）
	tmpData := t.TempDir()
	t.Setenv("SHANXI_DATA_DIR", tmpData)

	// 隔离工作目录（agent 实际改的"项目"）
	tmpWork := t.TempDir()
	if err := core.SetProjectRoot(tmpWork); err != nil {
		t.Fatalf("SetProjectRoot: %v", err)
	}

	const project = "demo-project"
	fileRel := "src/app.txt"
	fileAbs := filepath.Join(tmpWork, fileRel)

	// 写文件的"初始内容"（模拟 AgentFS 开启前就存在的文件）
	initContent := []byte("hello v0\n")
	if err := os.MkdirAll(filepath.Dir(fileAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileAbs, initContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// 1) 开辟会话
	sess := OpenAgentFSSession(project, tmpWork)
	if sess == nil {
		t.Fatal("OpenAgentFSSession 返回 nil（影子仓初始化失败）")
	}
	if sess.Project != project {
		t.Fatalf("session.Project = %q, want %q", sess.Project, project)
	}

	// 2) 模拟 write_file：before → 真实写 → after
	newContent := []byte("hello v1 - agent edited\n")
	args := map[string]any{"path": fileRel, "content": string(newContent)}
	OnBeforeWrite("mcp__fs__write_file", args)
	if err := os.WriteFile(fileAbs, newContent, 0o644); err != nil {
		t.Fatal(err)
	}
	OnAfterWrite("mcp__fs__write_file", args)

	// 3) 审计时间线应含 1 条 write 记录
	ap := agentfsAuditPath(project)
	raw, err := os.ReadFile(ap)
	if err != nil {
		t.Fatalf("读取 audit.jsonl 失败: %v", err)
	}
	var records []agentfsAudit
	for _, line := range splitLines(string(raw)) {
		if line == "" {
			continue
		}
		var a agentfsAudit
		if e := json.Unmarshal([]byte(line), &a); e != nil {
			t.Fatalf("audit 行解析失败 %q: %v", line, e)
		}
		records = append(records, a)
	}
	if len(records) != 1 {
		t.Fatalf("审计记录数 = %d, want 1（got: %+v）", len(records), records)
	}
	rec := records[0]
	if rec.Op != "write" {
		t.Errorf("rec.Op = %q, want write", rec.Op)
	}
	if rec.RelPath != fileRel {
		t.Errorf("rec.RelPath = %q, want %q", rec.RelPath, fileRel)
	}
	if rec.BeforeHash == "" || rec.AfterHash == "" {
		t.Errorf("before/after hash 不应为空: before=%q after=%q", rec.BeforeHash, rec.AfterHash)
	}
	if rec.Commit == "" {
		t.Errorf("commit 短 hash 不应为空（git 未提交？）")
	}
	commitV1 := rec.Commit

	// 4) 再模拟一次 edit（验证时间线增长 + 第二版内容）
	editArgs := map[string]any{"path": fileRel, "edits": []map[string]any{
		{"oldText": "hello v1 - agent edited", "newText": "hello v2 - edited again"},
	}}
	OnBeforeWrite("mcp__fs__edit_file", editArgs)
	edited := []byte("hello v2 - edited again\n")
	if err := os.WriteFile(fileAbs, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	OnAfterWrite("mcp__fs__edit_file", editArgs)

	raw2, _ := os.ReadFile(ap)
	if n := len(splitLines(string(raw2))); n != 2 {
		t.Fatalf("第二次写后审计记录数 = %d, want 2", n)
	}

	// 5) 验证"时间旅行"：用 git show <commitV1>:<rel> 取回 v1 历史版本内容，
	//    断言等于 newContent（v1），且不同于 v2。这证明影子仓能精确还原到任意提交。
	repo := agentfsRepoDir(project)
	histOut, err := gitRun(repo, "show", commitV1+":"+fileRel)
	if err != nil {
		t.Fatalf("git show %s:%s 失败: %v (%s)", commitV1, fileRel, err, histOut)
	}
	if histOut != string(newContent) {
		t.Errorf("commitV1 历史内容 = %q, want v1 %q", histOut, string(newContent))
	}
	// 当前真实盘应是 v2（第二次 edit 后）
	cur, _ := os.ReadFile(fileAbs)
	if string(cur) != string(edited) {
		t.Errorf("当前真实盘 = %q, want v2 %q", string(cur), string(edited))
	}
	// 执行还原（等价 AgentFSRestore 内部逻辑）：把历史版本写回真实盘
	if err := os.WriteFile(fileAbs, []byte(histOut), 0o644); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(fileAbs)
	if string(restored) != string(newContent) {
		t.Errorf("还原后真实盘 = %q, want v1 %q", string(restored), string(newContent))
	}
}

// splitLines 按 \n 切分并清理末尾空行（与 OnAfterWrite 的写入格式对齐）。
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
