package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
)

type IndexOutput struct {
	Files    map[string][]string `json:"files"`
	Inverted map[string][]string `json:"inverted"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: codex.exe <project_root>")
		os.Exit(1)
	}
	root := os.Args[1]

	output := IndexOutput{
		Files:    make(map[string][]string),
		Inverted: make(map[string][]string),
	}

	// Bleve 内存索引用于分词
	mapping := bleve.NewIndexMapping()
	idx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create index failed: %v\n", err)
		os.Exit(1)
	}
	defer idx.Close()

	filepath.Walk(root, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// 跳过目录与隐藏文件
		if info.IsDir() {
			name := strings.ToLower(info.Name())
			if name == ".git" || name == "node_modules" || name == "dist" || name == "vendor" || name == "__pycache__" || name == ".vscode" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") || info.Size() > 500*1024 {
			return nil
		}
		// 只看文本类文件
		ext := strings.ToLower(filepath.Ext(absPath))
		allowed := map[string]bool{".go": true, ".py": true, ".js": true, ".ts": true, ".cpp": true, ".h": true, ".c": true, ".java": true, ".rs": true, ".vue": true, ".md": true, ".json": true, ".yaml": true, ".toml": true}
		if !allowed[ext] {
			return nil
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil
		}
		text := string(content)
		relPath, _ := filepath.Rel(root, absPath)
		relPath = filepath.ToSlash(relPath)

		// 切片：按空行拆成段落（简单有效）
		paragraphs := strings.Split(text, "\n\n")
		var cleanParagraphs []string
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			if len(p) > 10 {
				cleanParagraphs = append(cleanParagraphs, p)
			}
		}
		output.Files[relPath] = cleanParagraphs

		// 构建倒排索引（利用 Bleve 分词）
		for i, para := range cleanParagraphs {
			doc := map[string]interface{}{
				"content": para,
				"path":    relPath,
				"idx":     i,
			}
			// 索引临时文档，只为提取 tokens
			idx.Index(fmt.Sprintf("%s:%d", relPath, i), doc)
		}
		return nil
	})

	// 从 Bleve 中导出倒排列表（简化版：直接扫文件重新构建 token->位置 映射）
	// 为避免依赖 Bleve 内部 API，我们采用手动分词（按空白 + 标点）
	// 实际上 Bleve 的分词可以通过分析器获取，但跨语言导出复杂。
	// 这里用纯 Go 实现简单分词，保证与 JS 行为一致。
	// 我们重建 inverted 索引：
	for relPath, paragraphs := range output.Files {
		for i, para := range paragraphs {
			tokens := tokenize(para)
			for _, tok := range tokens {
				tok = strings.ToLower(tok)
				// 过滤太短的词
				if len(tok) < 2 {
					continue
				}
				location := fmt.Sprintf("%s:%d", relPath, i)
				if !contains(output.Inverted[tok], location) {
					output.Inverted[tok] = append(output.Inverted[tok], location)
				}
			}
		}
	}

	// 输出 JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func tokenize(text string) []string {
	// 简单按非字母数字切分
	var tokens []string
	current := strings.Builder{}
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
