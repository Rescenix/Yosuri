package handler

import (
	"os"
	"path/filepath"
	"testing"

	"backend/internal/ai/core"
)

// TestAgentFSWriteEditRestore 端到端验证 AgentFS 本地历史时间线：
//  1. 开辟会话（OpenAgentFSSession）
//  2. 模拟一次 write_file：OnBeforeWrite → 真实写 → OnAfterWrite
//  3. 审计时间线出现该笔记录（AgentFSLog）
//  4. 用 historyStore 还原到 before 状态，真实盘被回退
//
// 全部用 t.TempDir()，不污染 ~/rescene_data，也不碰 re0 主仓库。
func TestAgentFSWriteEditRestore(t *testing.T) {
	// 隔离 AgentFS 数据根（指向临时目录，避免污染真实 rescene_data）
	tmpData := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", tmpData)

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
		t.Fatal("OpenAgentFSSession 返回 nil（历史目录初始化失败）")
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
	store := newHistoryStore(project)
	records, err := store.List("")
	if err != nil {
		t.Fatalf("读取审计日志失败: %v", err)
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
	if rec.BeforeBlob == "" {
		t.Errorf("before_blob 不应为空")
	}
	if !rec.ExistsBefore {
		t.Errorf("ExistsBefore 应为 true")
	}
	seqV1 := rec.Seq

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

	records2, err := store.List("")
	if err != nil {
		t.Fatalf("第二次写后读取审计日志失败: %v", err)
	}
	if len(records2) != 2 {
		t.Fatalf("第二次写后审计记录数 = %d, want 2", len(records2))
	}

	// 5) 验证"时间旅行"：用 historyStore 取回 seqV1 的 before 内容，
	//    应等于 newContent（v1），且不同于 v2。
	hist, err := store.Restore(seqV1)
	if err != nil {
		t.Fatalf("Restore seq=%d 失败: %v", seqV1, err)
	}
	if string(hist) != string(newContent) {
		t.Errorf("seqV1 before 内容 = %q, want v1 %q", string(hist), string(newContent))
	}
	// 当前真实盘应是 v2（第二次 edit 后）
	cur, _ := os.ReadFile(fileAbs)
	if string(cur) != string(edited) {
		t.Errorf("当前真实盘 = %q, want v2 %q", string(cur), string(edited))
	}
	// 执行还原：把 v1 写回真实盘
	if err := os.WriteFile(fileAbs, hist, 0o644); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(fileAbs)
	if string(restored) != string(newContent) {
		t.Errorf("还原后真实盘 = %q, want v1 %q", string(restored), string(newContent))
	}
}

// splitLines 按 \n 切分并清理末尾空行（与历史写入格式对齐）。
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
