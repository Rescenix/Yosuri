// Package memorydir 管理 ~/rescene_data/memory/ 目录下的多文件记忆。
//
// 结构：
//   memory/index.md       ← 无条件读取，轻量索引（只有 [[]] 文件链接 + 一句话摘要）
//   memory/preferences.md ← 按需读取
//   memory/project.md     ← 按需读取
//
// 读取时只读 index.md，然后根据当前任务用 bigram 匹配哪些行命中，
// 命中的行附带 [[]] 文件链接 → 读对应文件。
//
// 写入时 remember 工具写文件 + 自动更新 index.md 索引行。
package memorydir

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	linkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	wordRe = regexp.MustCompile(`[a-z0-9_]+`)
)

// IndexLine 索引中的一行
type IndexLine struct {
	File    string // [[file]] 的文件名部分（不含 .md）
	Summary string // 一行描述
	Raw     string // 原始行（用于渲染）
}

func path() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "memory")
}

func indexPath() string {
	return filepath.Join(path(), "index.md")
}

// ReadIndex 读取 index.md 的全部内容。
// 这是唯一无条件注入的部分，轻量（几十行）。
func ReadIndex() string {
	data, err := os.ReadFile(indexPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ParseIndex 解析 index.md 为结构化行。
func ParseIndex() []IndexLine {
	data, err := os.ReadFile(indexPath())
	if err != nil {
		return nil
	}
	var out []IndexLine
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := linkRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file := strings.TrimSpace(m[1])
		// 去掉 [[file]] 本身得到摘要
		summary := strings.TrimSpace(linkRe.ReplaceAllString(line, ""))
		summary = strings.TrimLeft(summary, "- \t")
		out = append(out, IndexLine{File: file, Summary: summary, Raw: line})
	}
	return out
}

// ReadWithLinks 读取 index.md + 根据 task 用 bigram 匹配命中行，
// 命中的行附带 [[]] 文件 → 读取对应文件内容。
// 返回拼接后的字符串（index.md + 命中的文件内容）。
func ReadWithLinks(task string) string {
	if task == "" {
		return ReadIndex()
	}

	lines := ParseIndex()
	if len(lines) == 0 {
		return ReadIndex()
	}

	// 对每行用 bigram 打分
	type scored struct {
		idx int
		ov  float64
		file string
	}
	var hits []scored
	qToks := norm(task)
	dir := path()

	for i, line := range lines {
		hay := line.Summary + " " + line.File
		ov := overlap(hay, task)
		if ov <= 0 {
			// 字面兜底
			hayLower := strings.ToLower(hay)
			for _, t := range qToks {
				if strings.Contains(hayLower, t) {
					ov = 0.1
					break
				}
			}
		}
		if ov > 0.15 {
			hits = append(hits, scored{i, ov, line.File})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].ov > hits[j].ov })

	// 最多取 3 个文件（预算控制）
	var parts []string
	parts = append(parts, "📇 记忆索引")
	parts = append(parts, ReadIndex())
	parts = append(parts, "")

	maxFiles := 3
	for i, h := range hits {
		if i >= maxFiles {
			break
		}
		filePath := filepath.Join(dir, h.file+".md")
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		parts = append(parts, "━━━ "+h.file+" ━━━")
		parts = append(parts, content)
	}

	return strings.Join(parts, "\n")
}

// Remember 写入一条记忆。
// file: 文件名（不含 .md），如 "preferences"
// summary: index.md 里该条后跟的摘要描述
// content: 文件正文
func Remember(file, summary, content string) error {
	dir := path()
	os.MkdirAll(dir, 0755)

	// 1. 写入/追加 content 到 file.md
	filePath := filepath.Join(dir, file+".md")
	existing := ""
	if data, err := os.ReadFile(filePath); err == nil {
		existing = strings.TrimSpace(string(data)) + "\n\n"
	}
	if err := os.WriteFile(filePath, []byte(existing+content+"\n"), 0644); err != nil {
		return err
	}

	// 2. 更新 index.md：如果有同 file 的行则覆盖，否则追加
	idxPath := indexPath()
	idxContent := ""
	if data, err := os.ReadFile(idxPath); err == nil {
		idxContent = string(data)
	}

	newLine := fmt.Sprintf("- [[%s]] %s", file, summary)
	if idxContent == "" {
		idxContent = "# 记忆索引\n\n" + newLine + "\n"
	} else {
		replaced := false
		var lines []string
		for _, line := range strings.Split(idxContent, "\n") {
			m := linkRe.FindStringSubmatch(line)
			if m != nil && strings.TrimSpace(m[1]) == file {
				// 替换行
				lines = append(lines, newLine)
				replaced = true
			} else {
				lines = append(lines, line)
			}
		}
		if replaced {
			idxContent = strings.Join(lines, "\n")
		} else {
			idxContent = strings.TrimRight(idxContent, "\n") + "\n" + newLine + "\n"
		}
	}
	return os.WriteFile(idxPath, []byte(idxContent+"\n"), 0644)
}

// ── bigram 选择器（精简版，与 swiftnet 同算法） ──

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
