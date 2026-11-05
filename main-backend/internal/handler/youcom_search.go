package handler

// youcom_search.go — You.com 联网搜索来源（web_search 的可选 mode 之一）
//
// 与 Bing 一样免 key、零配置即可用，但走 You.com 官方 MCP 端点：
//
//	https://api.you.com/mcp?profile=free（free profile，无需账号/API Key）
//
// 默认用免 key 的 free profile；如需切换到带认证的完整版，
// 设置环境变量 YOUCOM_MCP_URL=https://api.you.com/mcp 即可（Bearer YDC_API_KEY 鉴权由服务端处理）。
// 该端点是单次 JSON-RPC tools/call 即可返回结果的无状态流式 HTTP MCP，无需 initialize 握手。
//
// 结果为结构化数据（标题/链接/摘要），比 HTML 抓取稳定，返回格式与 bingSearch 对齐。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// defaultYoucomMCPURL 免 key 的 You.com MCP free profile（基础 you-search 工具）。
const defaultYoucomMCPURL = "https://api.you.com/mcp?profile=free"

// youcomMCPURL 优先读环境变量 YOUCOM_MCP_URL（可指向带认证的 https://api.you.com/mcp），默认免 key free profile。
func youcomMCPURL() string {
	if u := strings.TrimSpace(os.Getenv("YOUCOM_MCP_URL")); u != "" {
		return u
	}
	return defaultYoucomMCPURL
}

// youcomSearch 调 You.com MCP 的 you-search 工具，返回 (给模型的文本, 引用 URL 列表)。
// 免 key、零配置；与 bingSearch/firecrawlSearch 同构。
func youcomSearch(ctx context.Context, query string, limit int) (string, []string, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "you-search",
			"arguments": map[string]any{
				"query": query,
				"count": limit,
			},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, youcomMCPURL(), bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("You.com 联网搜索失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", nil, fmt.Errorf("You.com 联网搜索 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	// 响应是 SSE（text/event-stream），逐行取 "data: {...}"；也兼容直接返回 JSON 的场景。
	text, err := youcomExtractToolText(resp.Body)
	if err != nil {
		return "", nil, err
	}
	if text == "" {
		return "", nil, fmt.Errorf("You.com 联网搜索未返回可读结果")
	}
	return youcomFormatResults(query, text, limit)
}

// youcomSSEMessage SSE data 行 / 直接 JSON 共用的解析结构。
type youcomSSEMessage struct {
	Result *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// youcomExtractToolText 从响应体里取出 tools/call 的文本内容（第一段 text 即结果 JSON）。
func youcomExtractToolText(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var msg youcomSSEMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			continue
		}
		if msg.Error != nil && msg.Error.Message != "" {
			return "", fmt.Errorf("You.com 搜索失败：%s", msg.Error.Message)
		}
		if msg.Result == nil {
			continue
		}
		for _, c := range msg.Result.Content {
			if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
				return c.Text, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

// youcomFormatResults 解析 you-search 返回的结果 JSON，按 bingSearch 的输出格式整理。
func youcomFormatResults(query, text string, limit int) (string, []string, error) {
	var out struct {
		Results struct {
			Web []struct {
				URL         string `json:"url"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Contents    struct {
					Highlights []string `json:"highlights"`
				} `json:"contents"`
			} `json:"web"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return "", nil, fmt.Errorf("You.com 响应解析失败：%v", err)
	}
	items := out.Results.Web
	if len(items) == 0 {
		return fmt.Sprintf("联网搜索「%s」没有找到结果。可以换个说法再试。", query), nil, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "联网搜索「%s」结果（You.com）：\n", query)
	urls := make([]string, 0, len(items))
	for i, r := range items {
		if i >= limit {
			break
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = strings.TrimSpace(r.URL)
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n", i+1, title, strings.TrimSpace(r.URL))
		snippet := strings.TrimSpace(r.Description)
		if snippet == "" && len(r.Contents.Highlights) > 0 {
			snippet = strings.TrimSpace(r.Contents.Highlights[0])
		}
		if snippet != "" {
			sc := strings.Join(strings.Fields(snippet), " ")
			if len(sc) > 400 {
				sc = sc[:400] + "…"
			}
			sb.WriteString("   " + sc + "\n")
		}
		if u := strings.TrimSpace(r.URL); u != "" {
			urls = append(urls, u)
		}
	}
	return sb.String(), urls, nil
}
