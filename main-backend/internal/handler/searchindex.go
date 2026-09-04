package handler

// searchindex.go —— 纯 Go 内存倒排索引 + BM25 变体，零外部依赖。
//
// 为什么不用向量库：每个用户都得装模型服务（BGE 等），500 真实用户 = 500 份
// 伺候模型的破事，Windows 桌面端模型服务一崩 recall 就全挂。向量的收益（语义
// 召回）在"个人桌面端搜索自己聊过的会话"这个场景不值这个代价。
//
// 为什么比 FTS5 强：FTS5 对中文按空格/标点切词等于没切，搜"鉴权"要命中
// "token 校验"得靠用户恰好打中相同字面。这里用 memorydir 同款 bigram 分词
// （中文逐字二元组 + 英文小写词元），任何连续两字都能命中，召回面远大于
// FTS5；打分用 BM25 变体（词频 + IDF + 文档长度归一），相关度排序。
//
// 设计：SessionStore 持有一个 SessionSearchIndex，写路径（Append/Fork/Delete/
// UpsertWorkflowPair）增量维护；loadFromFile 全量重建。索引只存
// (token → sessionID → 词频/位置)，不存原文，搜索时回查 session 原文取片段。

import (
	"math"
	"strings"
	"unicode"
)

// postingsEntry 一个 token 在一个会话里的出现统计。
type postingsEntry struct {
	count int // 词频（BM25 的 tf 来源）
}

// SessionSearchIndex 内存倒排索引：token → sessionID → 出现统计。
// 非并发安全，调用方（SessionStore）负责持锁。
type SessionSearchIndex struct {
	// docLen sessionID → 消息总字数（BM25 文档长度归一化用）
	docLen map[string]int
	// inverted token → sessionID → 词频
	inverted map[string]map[string]*postingsEntry
	// totalDocs 已索引的会话数（BM25 IDF 用）
	totalDocs int
	// dirty 写路径置 true，下次搜索前懒重建（loadFromFile 后或索引被无效化时）
	dirty bool
}

// newSessionSearchIndex 新建空索引。
func newSessionSearchIndex() *SessionSearchIndex {
	return &SessionSearchIndex{
		docLen:   make(map[string]int),
		inverted: make(map[string]map[string]*postingsEntry),
	}
}

// tokenize 切成检索词元：中文逐字 bigram、英文按词小写、数字保留。
// bigram 是中文召回的关键：搜「鉴权」能命中「鉴权方案」，单字会全库泛匹配。
func tokenize(text string) []string {
	var cjk []rune
	var word []rune
	var toks []string
	flushCJK := func() {
		for i := 0; i < len(cjk); i++ {
			toks = append(toks, string(cjk[i])) // 单字兜底（长度1的查询）
			if i+1 < len(cjk) {
				toks = append(toks, string(cjk[i:i+2])) // bigram
			}
		}
		cjk = cjk[:0]
	}
	flushWord := func() {
		if len(word) > 0 {
			toks = append(toks, string(word))
			word = word[:0]
		}
	}
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			flushCJK()
			word = append(word, r)
		case r >= '\u4e00' && r <= '\u9fff':
			flushWord()
			cjk = append(cjk, r)
		default:
			flushCJK()
			flushWord()
		}
	}
	flushCJK()
	flushWord()
	// 去重保序
	seen := make(map[string]bool, len(toks))
	out := toks[:0]
	for _, t := range toks {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// isQueryToken 过滤查询里的纯标点/空白词元。
func isQueryToken(t string) bool {
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// addDoc 把一个会话的全部消息文本灌进索引（全量替换语义）。
// sid 已存在时先清旧倒排再写新的。
func (ix *SessionSearchIndex) addDoc(sid string, msgs []DSMessage) {
	ix.removeDoc(sid)
	n := 0
	for _, m := range msgs {
		text := m.Content
		if text == "" {
			continue
		}
		n += len([]rune(text))
		for _, t := range tokenize(text) {
			m := ix.inverted[t]
			if m == nil {
				m = make(map[string]*postingsEntry)
				ix.inverted[t] = m
			}
			if m[sid] == nil {
				m[sid] = &postingsEntry{}
			}
			m[sid].count++
		}
	}
	if n > 0 {
		ix.docLen[sid] = n
		ix.totalDocs++
	}
}

// removeDoc 把一个会话从索引里摘干净（Delete/重建前调用）。
func (ix *SessionSearchIndex) removeDoc(sid string) {
	if _, ok := ix.docLen[sid]; !ok {
		return
	}
	for tok, m := range ix.inverted {
		if _, hit := m[sid]; hit {
			delete(m, sid)
			if len(m) == 0 {
				delete(ix.inverted, tok)
			}
		}
	}
	delete(ix.docLen, sid)
	ix.totalDocs--
}

// bm25 常量：k1 控制词频饱和，b 控制文档长度归一强度。
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// search 查询索引：返回会话 ID → BM25 分，降序。多词元查询取 sum。
// avgLen 用全局平均；索引为空返回 nil。
func (ix *SessionSearchIndex) search(query string) map[string]float64 {
	qtoks := make([]string, 0, 8)
	for _, t := range tokenize(query) {
		if isQueryToken(t) {
			qtoks = append(qtoks, t)
		}
	}
	if len(qtoks) == 0 || ix.totalDocs == 0 {
		return nil
	}
	avgLen := 0
	for _, l := range ix.docLen {
		avgLen += l
	}
	avgLen /= ix.totalDocs
	if avgLen == 0 {
		avgLen = 1
	}

	scores := make(map[string]float64, 16)
	for _, qt := range qtoks {
		postings := ix.inverted[qt]
		if postings == nil {
			continue
		}
		// IDF：BM25 标准式，+1 防负值
		idf := log2(float64(ix.totalDocs-len(postings)) + 0.5)
		idf -= log2(float64(len(postings)) + 0.5)
		if idf < 0 {
			idf = 0.01
		}
		for sid, e := range postings {
			dl := float64(ix.docLen[sid])
			tf := float64(e.count)
			denom := tf + bm25K1*(1-bm25B+bm25B*dl/float64(avgLen))
			scores[sid] += idf * (tf * (bm25K1 + 1)) / denom
		}
	}
	return scores
}

// log2 以 2 为底的对数。
func log2(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return math.Log2(x)
}

// containsAllText 判断一组消息里是否有任一内容包含 token（小写不敏感）。
func containsAllText(msgs []DSMessage, token string) bool {
	if token == "" {
		return false
	}
	for _, m := range msgs {
		if strings.Contains(strings.ToLower(m.Content), token) {
			return true
		}
	}
	return false
}

