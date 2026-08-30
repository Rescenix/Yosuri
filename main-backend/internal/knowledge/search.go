package knowledge

// search.go —— 分块 + 索引 + 检索。
//
// 把每个文档切成 ~700 字符的片段（按段落边界切，尽量不断句），片段作为检索单元。
// 检索时对每个 chunk 做 bigram 重叠打分（与 memorydir 同算法），命中返回 top-k。
// 惰性缓存：按文件 mtime 判断是否重新切块，避免每轮都重新解析大文档。

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	chunkTarget  = 700 // 每段目标字符数
	chunkMax     = 1200
	defaultTopK  = 3 // 默认召回段数
	maxResultLen = 4000
)

// searchIndex 是进程内的惰性索引缓存：文件路径 -> 切好的块。
// 键是绝对路径，值存 mtime + chunks；mtime 变了就重新解析。
var (
	idxMu    sync.Mutex
	idxCache = map[string]cachedFile{}
)

type cachedFile struct {
	modTime int64
	chunks  []Chunk
}

// loadChunks 读取一个文件并切块，带缓存。
func loadChunks(path string) []Chunk {
	fi, err := os.Stat(path)
	if err != nil {
		return nil
	}
	mtime := fi.ModTime().Unix()

	idxMu.Lock()
	if c, ok := idxCache[path]; ok && c.modTime == mtime && c.chunks != nil {
		idxMu.Unlock()
		return c.chunks
	}
	idxMu.Unlock()

	text, err := extractText(path)
	if err != nil {
		return nil
	}
	chunks := splitChunks(path, text)

	idxMu.Lock()
	idxCache[path] = cachedFile{modTime: mtime, chunks: chunks}
	idxMu.Unlock()
	return chunks
}

// invalidateFile 清掉单个文件的缓存（用于重建索引）。
func invalidateFile(path string) {
	idxMu.Lock()
	delete(idxCache, path)
	idxMu.Unlock()
}

// invalidateAll 清空全部缓存（重建索引时调用）。
func invalidateAll() {
	idxMu.Lock()
	idxCache = map[string]cachedFile{}
	idxMu.Unlock()
}

// splitChunks 把整篇文本按段落边界切成定长片段。全程按 rune 处理，避免切坏中文。
func splitChunks(path string, text string) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	name := filepath.Base(path)

	var chunks []Chunk
	var cur []rune
	flush := func() {
		c := strings.TrimSpace(string(cur))
		if c != "" {
			chunks = append(chunks, Chunk{File: path, Name: name, Content: c})
		}
		cur = nil
	}

	for _, p := range splitParagraphs(text) {
		runes := []rune(p)
		for len(runes) > chunkMax {
			cut := bestCut(runes, chunkTarget)
			if cut <= 0 {
				cut = chunkTarget
			}
			cur = append(cur, runes[:cut]...)
			flush()
			runes = runes[cut:]
		}
		cur = append(cur, runes...)
		cur = append(cur, '\n')
		if len(cur) >= chunkTarget {
			flush()
		}
	}
	flush()
	return chunks
}

// splitParagraphs 按换行分成段落，保留非空段。
func splitParagraphs(text string) []string {
	lines := strings.Split(text, "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// bestCut 在 runes 里从 from 往回找最近断句点，返回截断位置（不含断句点之后）。
// 找不到返回 0（调用方回退到 chunkTarget）。
func bestCut(runes []rune, from int) int {
	if from > len(runes) {
		from = len(runes)
	}
	for i := from - 1; i >= from/2; i-- {
		switch runes[i] {
		case '。', '！', '？', '.', '!', '?', '；', ';':
			return i + 1
		}
	}
	return 0
}

// Search 检索知识库，返回与 query 最相关的片段（纯文本拼接，带来源标注）。
// 返回空串表示无命中或库为空。topK<=0 用默认值。
func Search(query string, topK int) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	if topK <= 0 {
		topK = defaultTopK
	}

	files := walkFiles()
	if len(files) == 0 {
		return ""
	}

	type hit struct {
		chunk Chunk
		score float64
	}
	var hits []hit
	for _, f := range files {
		for _, c := range loadChunks(f) {
			sc := overlap(c.Content, query)
			if sc > 0 {
				hits = append(hits, hit{c, sc})
			}
		}
	}
	if len(hits) == 0 {
		return ""
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })

	// 去重：同名文件只保留得分最高的一段（避免一个文件霸占所有名额）。
	seen := map[string]bool{}
	var parts []string
	for _, h := range hits {
		if seen[h.chunk.Name] {
			continue
		}
		seen[h.chunk.Name] = true
		parts = append(parts, "━━━ "+h.chunk.Name+" ━━━\n"+h.chunk.Content)
		if len(parts) >= topK {
			break
		}
	}
	return truncate(strings.Join(parts, "\n\n"), maxResultLen)
}

// Stats 返回库概况：文件数、总片段数。
func Stats() (files int, chunks int) {
	for _, f := range walkFiles() {
		files++
		chunks += len(loadChunks(f))
	}
	return
}

// truncate 按 rune 截断，避免切坏多字节中文。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// ── bigram 选择器（与 memorydir 同算法，复制一份避免跨包耦合） ──

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
	minLen := int(math.Min(float64(len(ba)), float64(len(bb))))
	union := len(ba) + len(bb) - inter
	contain := float64(inter) / float64(minLen)
	jacc := float64(inter) / float64(union)
	if contain > jacc {
		return contain
	}
	return jacc
}