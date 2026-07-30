package swiftnet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestNet(t *testing.T) *Net {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "MEMORY.md"))
}

func TestMemAppendAndSelect(t *testing.T) {
	n := newTestNet(t)

	res := n.MemAppend("Code模式四态机工作流走 GET /api/code/workflow", "CodeWork", "四态机/工作流/SSE")
	if !res.OK || res.ID == "" {
		t.Fatalf("append failed: %+v", res)
	}
	n.MemAppend("用户偏好先简单方案不过度设计", "UserBase", "偏好/简洁/KISS")

	hits := n.Select("四态机工作流是怎么实现的", 600, 0.12)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Cluster != "CodeWork" {
		t.Fatalf("wrong hit: %+v", hits[0])
	}

	// 无关查询不应命中
	if hits := n.Select("今天天气怎么样啊朋友", 600, 0.12); len(hits) != 0 {
		t.Fatalf("expected 0 hits for irrelevant query, got %d", len(hits))
	}
}

func TestProbeBeforeAppendMerges(t *testing.T) {
	n := newTestNet(t)
	// probe-before-append 针对的场景：同一事实被再次写入（关键词改述不同），
	// 应原位合并补充同义关键词，而不是新增重复行
	first := n.MemAppend("用户的风险偏好是保守型", "UserBase", "风险偏好/保守")
	second := n.MemAppend("用户的风险偏好是保守型", "UserBase", "风险厌恶/risk")

	if second.MergedID == "" {
		t.Fatalf("expected synonym merge, got new node: %+v", second)
	}
	if second.MergedID != first.ID {
		t.Fatalf("merged into wrong node: %s != %s", second.MergedID, first.ID)
	}
	// 合并后 keywords 应包含新同义词
	node, ok := n.Expand(first.ID)
	if !ok || !strings.Contains(node.Keywords, "risk") {
		t.Fatalf("keywords not merged: %+v", node)
	}
}

func TestPinInPlaceUpdate(t *testing.T) {
	n := newTestNet(t)
	n.Pin("P01", "UserBase", "旧身份")
	n.Pin("P01", "UserBase", "新身份")
	inject := n.UnconditionalInject()
	if strings.Contains(inject, "旧身份") || !strings.Contains(inject, "新身份") {
		t.Fatalf("pin should update in place: %s", inject)
	}
}

func TestPersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MEMORY.md")

	n1 := New(path)
	n1.Pin("P01", "UserBase", "身份一号")
	n1.HandoffWrite("上次做到一半的事")
	appended := n1.MemAppend("一条事实", "CodeWork", "事实/fact")

	// 重新加载同一文件
	n2 := New(path)
	stats := n2.Stats()
	if stats["pinned"] != 1 || stats["mem"] != 1 {
		t.Fatalf("reload lost data: %+v", stats)
	}
	if node, ok := n2.Expand(appended.ID); !ok || node.Text != "一条事实" {
		t.Fatalf("mem node not reloaded: %+v", node)
	}
	if !strings.Contains(n2.UnconditionalInject(), "上次做到一半的事") {
		t.Fatalf("handoff not reloaded")
	}

	// 文件格式检查：三个区标记都在（与 Python 版兼容的关键）
	data, _ := os.ReadFile(path)
	for _, zone := range []string{"[pinned]", "[handoff]", "[mem]"} {
		if !strings.Contains(string(data), zone) {
			t.Fatalf("zone %s missing in rendered file", zone)
		}
	}
}

func TestHandoffNeverGrows(t *testing.T) {
	n := newTestNet(t)
	long := strings.Repeat("这是很长的一行工作态记录需要被截断处理\n", 100)
	n.HandoffWrite(long)
	stats := n.Stats()
	if stats["file_tok"] > FileTokenCap {
		t.Fatalf("handoff exceeded budget")
	}
	n.mu.RLock()
	handoffTok := tok(n.handoff)
	n.mu.RUnlock()
	if handoffTok > HandoffBudget {
		t.Fatalf("handoff tok %d > budget %d", handoffTok, HandoffBudget)
	}
}
