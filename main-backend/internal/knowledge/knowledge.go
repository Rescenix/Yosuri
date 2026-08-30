// Package knowledge 管理 ~/rescene_data/knowledge/ 目录下的外挂知识库（RAG）。
//
// 与 memorydir 记忆互补：memorydir 是 agent 自己沉淀的长期记忆（小而精、每轮注入），
// knowledge 是用户丢进去的外部文档（md/txt/docx/pptx/pdf），体量大、不能全量注入，
// 只在对话命中的时候按需检索召回相关片段。
//
// 检索算法与 memorydir 同源（bigram 重叠打分），不引入向量数据库 —— 纯 Go、零外部
// 服务、离线可用。索引在内存里按文件 mtime 惰性重建，不落盘额外索引文件。
package knowledge

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var wordRe = regexp.MustCompile(`[a-z0-9_]+`)

// Dir 知识库根目录。用户往这里丢文档，agent 检索时只读这里。
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "knowledge")
}

// SupportedExts 支持的扩展名（小写，含点）。
var SupportedExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true,
	".docx": true, ".pptx": true, ".pdf": true,
}

// Chunk 一段可检索的文本片段。
type Chunk struct {
	File    string // 来源文件绝对路径
	Name    string // 展示名（相对 knowledge 根的文件名）
	Content string // 片段正文
}

// FileMeta 一个知识文件的元信息。
type FileMeta struct {
	Name    string // 相对 knowledge 根的路径
	Size    int64
	ModTime int64
	Chunks  int
}

// ListFiles 扫描知识库目录，返回支持格式的文件清单（按名称排序）。
func ListFiles() []FileMeta {
	dir := Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []FileMeta{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !SupportedExts[ext] {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, FileMeta{
			Name:    e.Name(),
			Size:    fi.Size(),
			ModTime: fi.ModTime().Unix(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// walkFiles 递归收集所有支持格式文件的绝对路径（含子目录）。
func walkFiles() []string {
	dir := Dir()
	var out []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if SupportedExts[strings.ToLower(filepath.Ext(p))] {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}