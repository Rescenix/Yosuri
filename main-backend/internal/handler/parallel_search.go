package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const parallelSearchURL = "https://search.parallel.ai/mcp"
const parallelSearchUserAgent = "Yosuri/1.0 (+https://github.com/Rescenix/Yosuri)"

type parallelSearchSessionKey struct{}

// Calls outside a chat workflow share one anonymous process session. Never
// generate a new identity per request or retry a quota error with another one.
var parallelSearchProcessSession = fmt.Sprintf("%x", sha256.Sum256([]byte(rand.Text())))

func parallelSearchSession(ctx context.Context) string {
	if id, _ := ctx.Value(parallelSearchSessionKey{}).(string); id != "" {
		return fmt.Sprintf("%x", sha256.Sum256([]byte("yosuri-search:"+id)))
	}
	return parallelSearchProcessSession
}

// parallelSearch is an opt-in native web_search source. It uses the existing
// Streamable HTTP transport with a fresh MCP connection per search, so separate
// workflows never share mutable transport state. No saved or ambient API keys
// are forwarded to the anonymous endpoint.
func parallelSearch(ctx context.Context, query string, limit int) (string, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	conn := &mcpConn{
		remoteURL:  parallelSearchURL,
		headers:    map[string]string{"User-Agent": parallelSearchUserAgent},
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	request := func(id int64, method string, params any) (json.RawMessage, error) {
		raw, err := conn.sendHTTPContext(ctx, id, method, params, 20*time.Second)
		if err != nil {
			return nil, fmt.Errorf("Parallel %s: %w", method, err)
		}
		return raw, nil
	}
	if _, err := request(1, "initialize", map[string]any{
		"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]string{"name": "yosuri", "version": "1.0.0"},
	}); err != nil {
		return "", nil, err
	}
	if _, err := request(0, "notifications/initialized", map[string]any{}); err != nil {
		return "", nil, err
	}
	raw, err := request(2, "tools/list", map[string]any{})
	if err != nil {
		return "", nil, err
	}
	found := false
	for _, def := range decodeMCPToolDefinitions(raw) {
		if def.Function.Name == "web_search" {
			found = true
		}
	}
	if !found {
		return "", nil, fmt.Errorf("Parallel MCP 未提供 web_search 工具")
	}
	// The MCP schema accepts objective and search_queries, not query/max_results.
	// max_results is applied locally to the provider's relevance-ordered results.
	raw, err = conn.sendHTTPContext(ctx, 3, "tools/call", map[string]any{
		"name": "web_search", "arguments": map[string]any{
			"objective": query, "search_queries": []string{query},
			"session_id": parallelSearchSession(ctx),
		},
	}, 60*time.Second)
	if err != nil {
		return "", nil, fmt.Errorf("Parallel 搜索失败: %w", err)
	}
	return formatParallelSearch(raw, query, limit)
}

func formatParallelSearch(raw json.RawMessage, query string, limit int) (string, []string, error) {
	var envelope struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
		Content           []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", nil, fmt.Errorf("Parallel MCP 响应解析失败: %w", err)
	}
	var texts []string
	for _, item := range envelope.Content {
		if item.Type == "text" {
			texts = append(texts, item.Text)
		}
	}
	if envelope.IsError {
		return "", nil, fmt.Errorf("Parallel 搜索失败: %s", truncateChars(strings.Join(texts, "\n"), 1000))
	}
	var result struct {
		Results *[]struct {
			URL      string   `json:"url"`
			Title    string   `json:"title"`
			Excerpts []string `json:"excerpts"`
		} `json:"results"`
	}
	payloads := append([]string{string(envelope.StructuredContent)}, texts...)
	for _, payload := range payloads {
		candidate := result
		if json.Unmarshal([]byte(payload), &candidate) == nil && candidate.Results != nil {
			result = candidate
			break
		}
	}
	if result.Results == nil {
		return "", nil, fmt.Errorf("Parallel MCP 响应缺少有效 results")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "联网搜索「%s」结果（Parallel）：\n", query)
	var urls []string
	for _, item := range *result.Results {
		u, err := url.Parse(strings.TrimSpace(item.URL))
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		urls = append(urls, u.String())
		fmt.Fprintf(&sb, "%d. %s\n   %s\n   %s\n", len(urls), item.Title, u.String(), truncateChars(strings.Join(item.Excerpts, "\n"), 2000))
		if len(urls) >= limit {
			break
		}
	}
	if len(urls) == 0 {
		sb.WriteString("未找到相关结果。\n")
	}
	return truncateChars(sb.String(), codeResultMaxChars), urls, nil
}
