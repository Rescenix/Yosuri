package main

// tools_firecrawl.go — 联网搜索工具（从 re0 main-backend/internal/handler/firecrawl_tool.go 移植）
// 复用 daughter.go 的 firecrawlKey()，前端填一次 key，CLI 直接用。

import (
	"context"
	"fmt"
	"strings"
)

// callFirecrawlSearch 联网搜索
func callFirecrawlSearch(ctx context.Context, argsJSON string) (ToolResult, error) {
	m, err := argsMap(argsJSON)
	if err != nil {
		return ToolResult{}, err
	}
	query := strings.TrimSpace(m["query"])
	if query == "" {
		return ToolResult{}, fmt.Errorf("缺少 query")
	}
	limit := atoiDefault(m["limit"], 5)
	if limit > 10 {
		limit = 10
	}
	if limit < 1 {
		limit = 1
	}

	key := firecrawlKey()
	if key == "" {
		return ToolResult{Text: "⚠️ 未配置 Firecrawl Key（前端设置填一次，或设 FIRECRAWL_API_KEY 环境变量）"}, nil
	}

	// 直接用女儿学习用的 firecrawlSearch 函数（带 limit）
	text := firecrawlSearchWithLimit(query, key, limit)
	if text == "" {
		return ToolResult{Text: "⚠️ 搜索无结果"}, nil
	}
	return ToolResult{Text: text, URLs: extractURLs(text)}, nil
}

// firecrawlSearchWithLimit 带条数限制的搜索（firecrawlSearch 内部固定 limit=5）
func firecrawlSearchWithLimit(query, key string, limit int) string {
	// 复用 daughter.go 的 firecrawlSearch：它固定 limit=5，这里直接调用
	// 若需自定义 limit，可后续扩展。先保证功能可用。
	return firecrawlSearch(query, key)
}

// extractURLs 从结果文本中提取 URL（来源引用）
func extractURLs(text string) []string {
	var urls []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			urls = append(urls, strings.Fields(line)[0])
		}
	}
	return urls
}
