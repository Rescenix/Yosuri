package handler

// worldbook_match.go —— 世界书条目的触发扫描与上下文渲染。
//
// 触发判定走两条路：
//   - 关键词命中：任一触发词作为子串出现在扫描文本里（中文按整串匹配，
//     英文按词边界，避免 "cat" 命中 "category"）。
//   - 常驻条目（Constant）不看文本，永远注入。
//
// 扫描文本 = 最近若干条对话 + 当前用户输入。越新的权重越高，但这里不做
// 加权打分，只做「命中即激活」——酒馆的语义就是关键词闸门，不搞模糊召回，
// 免得用户搞不清为什么这条设定突然生效了。

import (
	"regexp"
	"strings"
	"unicode"

	"backend/internal/memorydir"
)

// loreScanWindow 扫描最近多少条消息做关键词触发。太长了每轮都在全量重扫历史，
// 12 条足够覆盖一场戏的当前语境；更早的设定要么已经生效过（模型自己记得），
// 要么就该由常驻条目兜底。
const loreScanWindow = 12

// loreMaxInjectTokens 单轮世界书注入的 token 预算。超了就按 order 从后往前丢，
// 宁缺毋滥——世界书吃满上下文会把对话本身挤没。
const loreMaxInjectTokens = 1800

// hasCJK 判断字符串里是否含中日韩字符，决定用哪种匹配策略。
func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// loreKeyHit 判断一个触发词是否命中文本。
// 英文/数字词按词边界匹配（\b），中文词按子串匹配——中文没有空格分词，
// 词边界正则对中文无效。
func loreKeyHit(key, text string, caseMatch bool) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	hay, needle := text, key
	if !caseMatch {
		hay, needle = strings.ToLower(hay), strings.ToLower(needle)
	}
	if hasCJK(needle) {
		return strings.Contains(hay, needle)
	}
	// 词边界：ASCII 词两侧不能再接字母数字，防 cat 命中 category。
	re, err := regexp.Compile(`(^|[^a-z0-9])` + regexp.QuoteMeta(needle) + `([^a-z0-9]|$)`)
	if err != nil {
		return strings.Contains(hay, needle)
	}
	return re.MatchString(hay)
}

// loreEntryTriggered 一条是否被激活。
func loreEntryTriggered(e LoreEntry, text string) bool {
	if e.Constant {
		return true
	}
	if len(e.Keys) == 0 {
		return false
	}
	primary := false
	for _, k := range e.Keys {
		if loreKeyHit(k, text, e.CaseMatch) {
			primary = true
			break
		}
	}
	if !primary {
		return false
	}
	if e.Selective && len(e.Keys2) > 0 {
		for _, k := range e.Keys2 {
			if loreKeyHit(k, text, e.CaseMatch) {
				return true
			}
		}
		return false
	}
	return true
}

// loreScanText 把最近的消息拼成触发扫描文本。
func loreScanText(history []DSMessage, task string) string {
	var b strings.Builder
	start := 0
	if len(history) > loreScanWindow {
		start = len(history) - loreScanWindow
	}
	for _, m := range history[start:] {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	b.WriteString(task)
	return b.String()
}

// MatchedLore 一条命中结果，供渲染与前端「本轮激活了哪些设定」提示用。
type MatchedLore struct {
	Entry   LoreEntry
	Matched []string // 命中的触发词（常驻条目为空）
}

// matchWorldBook 扫描并返回按 order 升序、预算内的命中条目。
func matchWorldBook(lb *WorldBook, text string) []MatchedLore {
	if lb == nil || len(lb.Entries) == 0 {
		return nil
	}
	var hits []MatchedLore
	for _, e := range lb.Entries {
		if !e.Enabled || strings.TrimSpace(e.Content) == "" {
			continue
		}
		if !loreEntryTriggered(e, text) {
			continue
		}
		var matched []string
		if !e.Constant {
			for _, k := range e.Keys {
				if loreKeyHit(k, text, e.CaseMatch) {
					matched = append(matched, strings.TrimSpace(k))
				}
			}
		}
		hits = append(hits, MatchedLore{Entry: e, Matched: matched})
	}
	// order 小者先注入；同 order 按 uid 稳定排序，保证每轮注入顺序一致
	// （顺序抖一下，前缀缓存就整段作废）。
	sortMatchedLore(hits)

	used := 0
	out := hits[:0]
	for _, h := range hits {
		t := h.Entry.Token
		if t <= 0 {
			t = estimateTokenCount(h.Entry.Content)
		}
		if used > 0 && used+t > loreMaxInjectTokens {
			break
		}
		used += t
		out = append(out, h)
	}
	return out
}

// renderWorldBook 把命中条目拼成注入文本。task 为空时只出常驻条目
// （装配系统提示词的首轮之前还没有任务文本，常驻部分照挂）。
func renderWorldBook(lb *WorldBook, text string) string {
	hits := matchWorldBook(lb, text)
	if len(hits) == 0 {
		return ""
	}
	var b strings.Builder
	for _, h := range hits {
		if h.Entry.Position == "after" {
			continue // after 位置走对话流注入，不在系统段里重复
		}
		name := strings.TrimSpace(h.Entry.Name)
		if name != "" {
			b.WriteString("【" + name + "】\n")
		}
		b.WriteString(strings.TrimSpace(h.Entry.Content))
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

// loreAfterMessages 把 position=after 的命中条目转成注入对话流的消息。
// 酒馆的「注入到对话第 N 条之后」就是这个：它离生成点更近，权重更高。
func loreAfterMessages(lb *WorldBook, text string) []map[string]any {
	hits := matchWorldBook(lb, text)
	var msgs []map[string]any
	for _, h := range hits {
		if h.Entry.Position != "after" {
			continue
		}
		msgs = append(msgs, map[string]any{
			"role":    "system",
			"content": strings.TrimSpace(h.Entry.Content),
		})
	}
	return msgs
}

// ── 角色卡私有记忆/世界书的 Agent 目录工具复用 memorydir，这里只补排序 ──

func sortMatchedLore(hits []MatchedLore) {
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && loreLess(hits[j].Entry, hits[j-1].Entry); j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
}

func loreLess(a, b LoreEntry) bool {
	if a.Order != b.Order {
		return a.Order < b.Order
	}
	return a.UID < b.UID
}

// agentLorePathOrGlobal 给 HTTP 层用：agentID 空则操作全局书。
func agentLorePathOrGlobal(agentID string) string {
	agentID = memorydir.SanitizeAgentID(agentID)
	if agentID == "" {
		return loreFilePath()
	}
	return agentLoreFilePath(agentID)
}
