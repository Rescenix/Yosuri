package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withIsolatedSessionDir 比同包已有的 withTempDataDir 隔离得更彻底：
// migrateLegacyJSONFile 走的是 os.UserHomeDir() 而不是 RESCENE_DATA_DIR，
// 只设 RESCENE_DATA_DIR 的话每个用例都会去读用户真实家目录里的老 sessions.json
// 并把真实会话迁进临时目录，测试数据就被污染了。这里把家目录也一并指向临时目录。
func withIsolatedSessionDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dir)
	t.Setenv("USERPROFILE", dir) // Windows 下 os.UserHomeDir() 读这个
	t.Setenv("HOME", dir)        // 类 Unix 下读这个
}

// appendPair 追加一对 user+assistant，模拟一次完整往返（Append 只在往返成功时被调用）。
// Append 是异步落盘的，用例结尾若还有在途 goroutine，Windows 上会撞到
// TempDir 清理与 os.Rename 打架；settle 用来等它们落完。
func appendPair(t *testing.T, s *SessionStore, sid, userText, botText string) {
	t.Helper()
	s.Append(sid, DSMessage{Role: "user", Content: userText})
	s.Append(sid, DSMessage{Role: "assistant", Content: botText})
}

// settle 等异步落盘的 goroutine 收尾再让用例结束。Append 是 fire-and-forget
// （go persistAll()，无生命周期句柄），所以只能先让出时间片给那些已经派生、
// 但可能还没跑到 persistAll 的 goroutine，再同步落一次盘把队列排空——
// 否则 Windows 上迟到的 tmp+rename 会跟 TempDir 清理打架（Access is denied /
// directory is not empty）。纯测试用，不为它改生产代码的落盘策略。
func settle(s *SessionStore) {
	time.Sleep(50 * time.Millisecond)
	_ = s.persistAll()
}

func titleOf(t *testing.T, s *SessionStore, sid string) string {
	t.Helper()
	for _, info := range s.List() {
		if info.ID == sid {
			return info.Title
		}
	}
	return "<未列出>"
}

// 分叉之后父会话必须逐字完好——这是整个分支功能存在的理由，
// 以前的 Truncate 正是在这里把用户的原始线索永久毁掉的。
func TestForkKeepsParentIntact(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })

	appendPair(t, s, "p", "写一个登录页", "好的")
	appendPair(t, s, "p", "加个记住我", "加好了")

	childID, ok := s.Fork("p", 2)
	if !ok {
		t.Fatal("Fork 应当成功")
	}

	child := s.Get(childID)
	if len(child) != 2 || child[0].Content != "写一个登录页" {
		t.Fatalf("分支应拷到前 2 条，实得 %d 条: %+v", len(child), child)
	}

	// 往分支里写，父会话不能被牵连（切片别名 bug 会在这里暴露）
	appendPair(t, s, childID, "改成注册页", "改好了")
	if parent := s.Get("p"); len(parent) != 4 {
		t.Fatalf("父会话应仍有 4 条，实得 %d 条", len(parent))
	}
	if s.Get("p")[2].Content != "加个记住我" {
		t.Error("父会话第 3 条被分支写坏了")
	}
}

func TestForkKeepClampingAndEmptyParent(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	appendPair(t, s, "p", "hi", "hello")

	if id, _ := s.Fork("p", 999); len(s.Get(id)) != 2 {
		t.Errorf("keep 超长应钳到全长 2，实得 %d", len(s.Get(id)))
	}
	if id, _ := s.Fork("p", -1); len(s.Get(id)) != 0 {
		t.Errorf("keep 为负应钳到 0，实得 %d", len(s.Get(id)))
	}
	if _, ok := s.Fork("根本不存在", 2); ok {
		t.Error("父会话不存在时应返回 ok=false")
	}
}

// 元数据必须真的落盘：重建一个 store 读同一个文件，血缘还得在。
func TestForkMetadataRoundTrip(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	appendPair(t, s, "p", "原始任务", "回复")
	childID, _ := s.Fork("p", 2) // Fork 是同步落盘的，之后可以直接重建

	reloaded := NewSessionStore(ChatSessionsDomain)
	if got := reloaded.ForkIndex(childID); got != 2 {
		t.Fatalf("重建后 ForkIndex 应为 2，实得 %d", got)
	}
	var found bool
	for _, info := range reloaded.List() {
		if info.ID == childID {
			found = true
			if info.ParentID != "p" {
				t.Errorf("重建后 ParentID 应为 p，实得 %q", info.ParentID)
			}
		}
	}
	if !found {
		t.Error("重建后分支没出现在 List() 里")
	}
}

// 老的磁盘格式没有 parent_id/fork_index 两个键，必须照旧当根会话加载，行为零变化。
func TestLoadLegacyRecordWithoutForkFields(t *testing.T) {
	withIsolatedSessionDir(t)
	legacy := `{"sess_old":{"messages":[{"role":"user","content":"老会话的第一句","timestamp":"2026-01-01T00:00:00Z"}],"compress_index":0}}`
	if err := os.WriteFile(sessionsFilePath(ChatSessionsDomain), []byte(legacy), 0644); err != nil {
		t.Fatalf("写测试文件失败: %v", err)
	}

	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	if got := titleOf(t, s, "sess_old"); got != "老会话的第一句" {
		t.Errorf("老记录标题应按原行为取第一条用户消息，实得 %q", got)
	}
	for _, info := range s.List() {
		if info.ID == "sess_old" && info.ParentID != "" {
			t.Errorf("老记录应是根会话，实得 ParentID=%q", info.ParentID)
		}
	}
}

// 老记录重写一遍之后不该平白多出 parent_id / fork_index 两个键（omitempty 的意义）。
func TestRootSessionsDoNotGainForkKeysOnRewrite(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	appendPair(t, s, "root", "只是普通会话", "嗯")
	if err := s.persistAll(); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}

	data, err := os.ReadFile(sessionsFilePath(ChatSessionsDomain))
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	var records map[string]map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if _, has := records["root"]["parent_id"]; has {
		t.Error("根会话不该写出 parent_id 键")
	}
	if _, has := records["root"]["fork_index"]; has {
		t.Error("根会话不该写出 fork_index 键")
	}
}

// 标题规则：分支取分岐点之后的第一条用户消息，父会话仍取全局第一条。
// 从 0 开始扫的话两者会完全一样，侧边栏就分不出谁是谁。
func TestBranchTitleUsesFirstPostForkUserMessage(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	appendPair(t, s, "p", "写一个登录页", "好的")

	childID, _ := s.Fork("p", 2)
	appendPair(t, s, childID, "改成注册页", "改好了")

	if got := titleOf(t, s, "p"); got != "写一个登录页" {
		t.Errorf("父会话标题应不变，实得 %q", got)
	}
	if got := titleOf(t, s, childID); got != "改成注册页" {
		t.Errorf("分支标题应取分岐点后第一条，实得 %q（说明还在从 0 扫）", got)
	}
}

// 刚分叉、还没跑第一轮时标题回落到占位，绝不能继承父标题
// ——继承的话就退回到"所有分支标题一模一样"这个要解决的问题本身。
func TestFreshForkTitleFallsBackToPlaceholder(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	appendPair(t, s, "p", "父会话标题", "回复")

	childID, _ := s.Fork("p", 2)
	if got := titleOf(t, s, childID); got == "父会话标题" {
		t.Error("刚分叉的分支不该继承父标题")
	} else if got != "新对话" {
		t.Errorf("应回落到占位标题，实得 %q", got)
	}
}

// 删父会话时子分支升为根，而不是级联删除或变成指向虚空的孤儿。
func TestDeleteParentPromotesChildrenToRoots(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	appendPair(t, s, "p", "父任务", "回复")
	childID, _ := s.Fork("p", 2)
	appendPair(t, s, childID, "分支任务", "回复")

	titleBefore := titleOf(t, s, childID)
	s.Delete("p")

	var found bool
	for _, info := range s.List() {
		if info.ID == childID {
			found = true
			if info.ParentID != "" {
				t.Errorf("父会话已删，子分支应升为根，实得 ParentID=%q", info.ParentID)
			}
		}
	}
	if !found {
		t.Fatal("删父会话把子分支也弄没了——分支应当独立存活")
	}
	if len(s.Get(childID)) != 4 {
		t.Errorf("子分支历史应完好（含拷来的前缀），实得 %d 条", len(s.Get(childID)))
	}
	// 升为根不该让分支改名：ForkIndex 要留着，否则标题会跳回拷贝来的前缀第一句
	if got := titleOf(t, s, childID); got != titleBefore {
		t.Errorf("父会话被删后分支标题不该变：%q → %q", titleBefore, got)
	}
	// 而且重启（重新从磁盘加载）之后也得稳住
	settle(s)
	reloaded := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(reloaded) })
	if got := titleOf(t, reloaded, childID); got != titleBefore {
		t.Errorf("重启后分支标题不该变：%q → %q", titleBefore, got)
	}
}

// 回归测试：store 的落盘路径必须在构造时就定死，绝不能每次写盘现读 RESCENE_DATA_DIR。
//
// 之前它是每次现算的，而 Append 的落盘是 fire-and-forget 的 goroutine——测试结束、
// t.Setenv 把环境变量恢复之后，那些迟到的 goroutine 就会按恢复后的环境重新算路径，
// 把测试用的内存状态写进用户真实的 ~/rescene_data/。这真的发生过，把用户 340KB 的
// 会话记录覆盖成了 1KB 的测试数据。
func TestStorePathIsPinnedAtConstruction(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	pinned := s.path

	// 模拟"测试结束、环境变量被恢复"：换一个完全不同的数据目录
	elsewhere := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", elsewhere)
	t.Setenv("USERPROFILE", elsewhere)
	t.Setenv("HOME", elsewhere)

	appendPair(t, s, "s1", "环境变量换了之后写的", "回复")
	if err := s.persistAll(); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}

	if s.path != pinned {
		t.Errorf("落盘路径不该跟着环境变量变：%q → %q", pinned, s.path)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "sessions_"+ChatSessionsDomain+".json")); err == nil {
		t.Error("改了环境变量后又往新目录写了——迟到的落盘 goroutine 会污染用户真实数据")
	}
}

// 审批规则要跟着分支走，但必须是克隆——共享 map 的话子会话改规则会静默改写父会话的权限。
func TestForkClonesApprovalRules(t *testing.T) {
	withIsolatedSessionDir(t)
	s := NewSessionStore(ChatSessionsDomain)
	t.Cleanup(func() { settle(s) })
	appendPair(t, s, "p", "任务", "回复")
	s.SetApprovalRule("p", "approve:mcp__shell__run", true)

	childID, _ := s.Fork("p", 2)
	if !s.GetApprovalRule(childID, "approve:mcp__shell__run") {
		t.Fatal("分支应继承父会话的免审批规则")
	}

	s.SetApprovalRule(childID, "approve:mcp__shell__run", false)
	if !s.GetApprovalRule("p", "approve:mcp__shell__run") {
		t.Error("改分支的规则把父会话的也改了——说明是共享引用而非克隆")
	}
}
