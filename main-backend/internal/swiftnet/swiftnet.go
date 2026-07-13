// Package swiftnet 是 AAP 雨燕神经网络（SwiftNet）单文件记忆引擎的 Go 移植。
//
// 设计判断来自 AAP/aap/memory/swiftnet.py（用户 2026-07-11 基于 benchmark 判定）：
// 三种生命周期不同的记忆不该共用一个召回机制——
//
//	[pinned]  身份       — 无条件注入，只改不删，≤150 tok
//	[handoff] 工作态     — 无条件注入，整段重写永不膨胀，≤200 tok
//	[inbox]   跨agent收件 — 无条件注入，不靠语义召回
//	[mem]     事实库     — bigram 选择器召回，probe-before-append 写侧防重
//
// 文件格式与 Python 版逐字节兼容（同一个 MEMORY.md 两边都能读写），
// 将来可直接走 AAP MemorySync 做跨机同步。
// 选择器零模型依赖：中文按单字、英文按 [a-z0-9_]+ 分词后取 bigram，
// containment-biased 相似度（短句被长句包含也应高相似）。
package swiftnet

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// 注入预算（与 Python 版一致）
const (
	PinnedBudget  = 150
	HandoffBudget = 200
	ProbeBudget   = 300
	FileTokenCap  = 50000 // 文件级硬上限，超出拒绝追加并要求 consolidate
)

type Node struct {
	ID       string
	Cluster  string
	Keywords string
	Text     string
	TS       float64
}

type Net struct {
	memFile string
	mu      sync.RWMutex
	pinned  []Node // pinned 不用 Keywords 字段
	handoff string
	mem     []Node
	inbox   []Node
}

// ==================== 单例 ====================

var (
	defaultOnce sync.Once
	defaultNet  *Net
)

// Default 返回进程级单例。路径：AURORA_MEMORY_FILE 环境变量，
// 默认 ~/shanxi_data/MEMORY.md（与旧 memory.json 同目录）。
func Default() *Net {
	defaultOnce.Do(func() {
		path := os.Getenv("AURORA_MEMORY_FILE")
		if path == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				home = "."
			}
			path = filepath.Join(home, "shanxi_data", "MEMORY.md")
		}
		defaultNet = New(path)
	})
	return defaultNet
}

func New(memFile string) *Net {
	n := &Net{memFile: memFile}
	os.MkdirAll(filepath.Dir(memFile), 0755)
	n.load()
	return n
}

// ==================== 选择器（零模型 bigram） ====================

var wordRe = regexp.MustCompile(`[a-z0-9_]+`)

func norm(s string) []string {
	s = strings.ToLower(s)
	toks := wordRe.FindAllString(s, -1)
	for _, r := range s {
		if r >= '一' && r <= '鿿' {
			toks = append(toks, string(r))
		}
	}
	return toks
}

func bigrams(s string) map[[2]string]bool {
	t := norm(s)
	out := map[[2]string]bool{}
	for i := 0; i+1 < len(t); i++ {
		out[[2]string{t[i], t[i+1]}] = true
	}
	return out
}

// overlap 0..1，containment-biased（非对称 Jaccard）：
// 短句几乎被长句覆盖即≈1，适配"同义改述"合并。
func overlap(a, b string) float64 {
	ba, bb := bigrams(a), bigrams(b)
	if len(ba) == 0 && len(bb) == 0 {
		if strings.TrimSpace(a) == strings.TrimSpace(b) {
			return 1.0
		}
		return 0.0
	}
	if len(ba) == 0 || len(bb) == 0 {
		return 0.0
	}
	inter := 0
	for k := range ba {
		if bb[k] {
			inter++
		}
	}
	minLen := len(ba)
	if len(bb) < minLen {
		minLen = len(bb)
	}
	union := len(ba) + len(bb) - inter
	contain := float64(inter) / float64(minLen)
	jacc := float64(inter) / float64(union)
	if contain > jacc {
		return contain
	}
	return jacc
}

func tok(s string) int {
	n := len(s) / 3
	if n < 1 {
		return 1
	}
	return n
}

// ==================== 文件解析/渲染（与 Python 版格式兼容） ====================

var (
	zoneRe = regexp.MustCompile(`^\[(pinned|handoff|mem|inbox)\]\s*(.*)$`)
	pinRe  = regexp.MustCompile(`(P\d+)\|(\w+)\|(.*)`)
	memRe  = regexp.MustCompile(`(0x[0-9a-f]+)\|(\w+)\|([^|]*)\|(.*)`)
)

func parseMemLine(s string) Node {
	if m := memRe.FindStringSubmatch(s); m != nil {
		return Node{ID: m[1], Cluster: m[2], Keywords: strings.TrimSpace(m[3]), Text: strings.TrimSpace(m[4])}
	}
	return Node{ID: fmt.Sprintf("0x%x", hashStr(s)&0xffff), Cluster: "mem", Text: s}
}

func (n *Net) load() {
	data, err := os.ReadFile(n.memFile)
	if err != nil {
		return
	}
	cur := ""
	var handoffLines []string
	for _, line := range strings.Split(string(data), "\n") {
		if m := zoneRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			cur = m[1]
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch cur {
		case "pinned":
			if trimmed != "" {
				if m := pinRe.FindStringSubmatch(trimmed); m != nil {
					n.pinned = append(n.pinned, Node{ID: m[1], Cluster: m[2], Text: strings.TrimSpace(m[3])})
				}
			}
		case "handoff":
			handoffLines = append(handoffLines, line)
		case "mem":
			if trimmed != "" {
				n.mem = append(n.mem, parseMemLine(trimmed))
			}
		case "inbox":
			if trimmed != "" {
				n.inbox = append(n.inbox, parseMemLine(trimmed))
			}
		}
	}
	n.handoff = strings.TrimSpace(strings.Join(handoffLines, "\n"))
}

func (n *Net) render() string {
	var out []string
	out = append(out, "# MEMORY.md — 雨燕神经网络：一个文件，四个区，一个写入者", "")
	out = append(out, fmt.Sprintf("[pinned] ← 无条件注入，≤%d tok，只改不删", PinnedBudget))
	for _, p := range n.pinned {
		out = append(out, fmt.Sprintf("%s|%s|%s", p.ID, p.Cluster, p.Text))
	}
	out = append(out, "")
	out = append(out, fmt.Sprintf("[handoff] ← 无条件注入，会话末尾整段重写，硬上限 %d tok", HandoffBudget))
	if n.handoff != "" {
		out = append(out, n.handoff)
	}
	out = append(out, "")
	out = append(out, "[inbox] ← 跨agent收件箱，无条件注入，不靠语义召回（解决'每次都要说'）")
	for _, m := range n.inbox {
		out = append(out, fmt.Sprintf("%s|%s|%s|%s", m.ID, m.Cluster, m.Keywords, m.Text))
	}
	out = append(out, "")
	out = append(out, "[mem] ← 选择器召回，追加+原位touch；Expand 按需")
	for _, m := range n.mem {
		out = append(out, fmt.Sprintf("%s|%s|%s|%s", m.ID, m.Cluster, m.Keywords, m.Text))
	}
	return strings.Join(out, "\n") + "\n"
}

func (n *Net) save() {
	tmp := n.memFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(n.render()), 0644); err != nil {
		return
	}
	os.Rename(tmp, n.memFile)
}

func hashStr(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func newID(text string) string {
	return fmt.Sprintf("0x%x", (uint32(time.Now().Unix())&0xffff)^(hashStr(text)&0xffff))
}

// ==================== 写：四区 API ====================

// Pin 身份记忆：无条件注入，只改不删。pid 形如 P01/P02。
func (n *Net) Pin(pid, cluster, text string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for i := range n.pinned {
		if n.pinned[i].ID == pid {
			n.pinned[i].Text = text // 原位改，不新增
			n.save()
			return
		}
	}
	n.pinned = append(n.pinned, Node{ID: pid, Cluster: cluster, Text: text})
	n.save()
}

// HandoffWrite 工作态：整段重写，不追加，永不膨胀。
func (n *Net) HandoffWrite(block string) {
	block = strings.TrimSpace(block)
	if tok(block) > HandoffBudget {
		lines := strings.Split(block, "\n")
		for len(lines) > 0 && tok(strings.Join(lines, "\n")) > HandoffBudget {
			lines = lines[1:] // 保留最近
		}
		block = strings.Join(lines, "\n")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.handoff = block
	n.save()
}

type AppendResult struct {
	OK       bool
	MergedID string // 非空表示 probe 命中同义，原位合并未新增
	ID       string
	Err      string
}

// MemAppend 事实库：probe-before-append 防重 + 文件级预算 cap。
// keywords 建议写入时铺同义改述（如 "风险偏好/风险厌恶/risk appetite"）。
func (n *Net) MemAppend(text, cluster, keywords string) AppendResult {
	text = strings.TrimSpace(text)
	if text == "" {
		return AppendResult{Err: "empty text"}
	}
	if cluster == "" {
		cluster = "mem"
	}
	kw := keywords
	if kw == "" {
		kw = text
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// probe-before-append：自查重，超阈值原位合并不新增行
	bestIdx, bestOv := -1, 0.0
	splitRe := regexp.MustCompile(`[/,，、]`)
	nk := map[string]bool{}
	for _, k := range splitRe.Split(kw, -1) {
		if k = strings.TrimSpace(k); k != "" {
			nk[k] = true
		}
	}
	for i, m := range n.mem {
		ov := overlap(m.Keywords+" "+m.Text, kw+" "+text)
		mk := map[string]bool{}
		for _, k := range splitRe.Split(m.Keywords, -1) {
			if k = strings.TrimSpace(k); k != "" {
				mk[k] = true
			}
		}
		inter, union := 0, len(nk)
		for k := range mk {
			if nk[k] {
				inter++
			} else {
				union++
			}
		}
		if union > 0 {
			if kwOv := float64(inter) / float64(union); kwOv > ov {
				ov = kwOv
			}
		}
		if ov > bestOv {
			bestIdx, bestOv = i, ov
		}
	}
	if bestIdx >= 0 && bestOv > 0.6 {
		// 原位合并：补 keywords 同义，不新增行
		merged := n.mem[bestIdx].Keywords
		for k := range nk {
			if !strings.Contains(merged, k) {
				if merged == "" {
					merged = k
				} else {
					merged += "/" + k
				}
			}
		}
		n.mem[bestIdx].Keywords = merged
		n.save()
		return AppendResult{OK: true, MergedID: n.mem[bestIdx].ID}
	}

	if tok(n.render())+tok(text) > FileTokenCap {
		return AppendResult{Err: "budget_exceeded: 记忆文件超过硬上限，需要 consolidate 复核"}
	}
	node := Node{ID: newID(text), Cluster: cluster, Keywords: kw, Text: text, TS: float64(time.Now().Unix())}
	n.mem = append(n.mem, node)
	n.save()
	return AppendResult{OK: true, ID: node.ID}
}

// InboxWrite 跨 agent 收件箱：无条件注入，不靠语义召回。
func (n *Net) InboxWrite(text, fromAgent string) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	node := Node{ID: newID(text), Cluster: "from:" + fromAgent, Keywords: text, Text: strings.TrimSpace(text)}
	n.inbox = append(n.inbox, node)
	n.save()
	return node.ID
}

// ==================== 读：注入与召回 ====================

// UnconditionalInject 返回无条件注入的文本块（pinned + handoff + inbox），
// 供聊天系统提示词直接拼接；空记忆时返回空串。
func (n *Net) UnconditionalInject() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	var parts []string
	if len(n.pinned) > 0 {
		var lines []string
		for _, p := range n.pinned {
			lines = append(lines, "• "+p.Text)
		}
		parts = append(parts, "# 身份（常驻）\n"+strings.Join(lines, "\n"))
	}
	if n.handoff != "" {
		parts = append(parts, "# 工作态（上次会话交接）\n"+n.handoff)
	}
	if len(n.inbox) > 0 {
		var lines []string
		for _, m := range n.inbox {
			lines = append(lines, fmt.Sprintf("• [%s] %s", m.Cluster, m.Text))
		}
		parts = append(parts, "# 跨agent收件箱\n"+strings.Join(lines, "\n"))
	}
	return strings.Join(parts, "\n\n")
}

// Select 选择器召回：按 budget（token）严格截断，minOverlap 入选门槛。
func (n *Net) Select(query string, budget int, minOverlap float64) []Node {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if strings.TrimSpace(query) == "" || len(n.mem) == 0 {
		return nil
	}
	type scored struct {
		ov float64
		m  Node
	}
	var hits []scored
	qToks := norm(query)
	for _, m := range n.mem {
		hay := m.Keywords + " " + m.Text
		ov := overlap(hay, query)
		if ov <= 0 {
			// 字面兜底：query 任一 token 出现在 hay 里给最低分
			hayLower := strings.ToLower(hay)
			for _, t := range qToks {
				if strings.Contains(hayLower, t) {
					ov = 0.1
					break
				}
			}
		}
		if ov > minOverlap {
			hits = append(hits, scored{ov, m})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].ov > hits[j].ov })
	var out []Node
	used := 0
	for _, h := range hits {
		line := fmt.Sprintf("%s|%s|%s", h.m.ID, h.m.Cluster, truncRunes(h.m.Text, 60))
		if used+tok(line) > budget {
			break
		}
		out = append(out, h.m)
		used += tok(line)
	}
	return out
}

// Expand 两级召回：按 id 拉全量。
func (n *Net) Expand(id string) (Node, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	for _, m := range n.mem {
		if m.ID == id {
			return m, true
		}
	}
	return Node{}, false
}

// Stats 供健康检查/调试。
func (n *Net) Stats() map[string]int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return map[string]int{
		"pinned":   len(n.pinned),
		"mem":      len(n.mem),
		"inbox":    len(n.inbox),
		"file_tok": tok(n.render()),
	}
}

func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
