package handler

// youcom_tool.go — 联网搜索工具：You.com（免 key 的 ?profile=free MCP 端点）
//
// youcom 是 web_search 的可选来源之一（设置面板 → 模型 → 联网来源 → You.com）。
// 默认走免 key 的公共 MCP 端点 https://api.you.com/mcp?profile=free（基础 you-search，
// 每天约 100 次），与 Bing 兜底同一「零配置」定位：不填 Key、不注册账号即可联网。
// 设置环境变量 YDC_API_KEY 后切换到认证端点 https://api.you.com/mcp
// （Key 在 you.com/platform/api-keys 获取，工具与额度更多）。
//
// 协议：Streamable HTTP JSON-RPC（与 mcp_client.go 的远程 transport 同款握手）：
// initialize → notifications/initialized → tools/call you-search。
// You.com 实测（2026-09-30）：服务端无状态，initialize 不下发 session id，
// 每次搜索独立完成一次握手即可，不需要长连接；应答可能是 application/json，
// 也可能是 text/event-stream——SSE 下会先推一条 notifications/message 进展通知，
// 再推真正的应答，因此这里收集全部 data 事件后取带 result 的那条
// （mcp_client.go 的 readFirstSSEData 只取第一条，不适用于 You.com 的双事件流）。

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

// youcomFreeMCPURL / youcomAuthenticatedURL 是 var 而非 const：单测里换成
// httptest 的假 MCP server（与 officialMCPRegistryBaseURL 同一手法）。
var (
	youcomFreeMCPURL       = "https://api.you.com/mcp?profile=free"
	youcomAuthenticatedURL = "https://api.you.com/mcp"
)

const youcomProtocolVersion = "2025-06-18"

// youcomAPIKey 环境变量 YDC_API_KEY（可选）。设置后走认证端点，否则免 key。
func youcomAPIKey() string {
	return strings.TrimSpace(os.Getenv("YDC_API_KEY"))
}

// youcomContentItem 是 tools/call 应答 content 里的一段；you-search 的结果
// JSON 就在第一段 text 里（形如 {"results":{"web":[{url,title,description}...]}}）。
type youcomContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// youcomWebResult you-search 返回的一条网页结果。
type youcomWebResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// youcomMCPRequest 对 You.com MCP 端点发一次 JSON-RPC 请求，返回应答的 result
// 原始 JSON。id<=0 表示通知（202 Accepted 无应答，返回 nil）。
func youcomMCPRequest(ctx context.Context, client *http.Client, endpoint, apiKey, method string, id int64, params any) (json.RawMessage, error) {
	payload := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id > 0 {
		payload["id"] = id
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", youcomProtocolVersion)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if id <= 0 && resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(data))
		if len(message) > 300 {
			message = message[:300]
		}
		return nil, fmt.Errorf("You.com MCP 返回 HTTP %d：%s", resp.StatusCode, message)
	}
	var messages []json.RawMessage
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		messages, err = youcomReadSSEMessages(resp.Body, 8<<20)
		if err != nil {
			return nil, err
		}
	} else {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		messages = []json.RawMessage{bytes.TrimSpace(data)}
	}
	// 从全部消息里取带 result/error 的应答（跳过 notifications/message 进展通知）
	for _, msg := range messages {
		var rpc struct {
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(msg, &rpc) != nil {
			continue
		}
		if len(rpc.Error) > 0 && string(rpc.Error) != "null" {
			return nil, fmt.Errorf("You.com MCP 错误: %s", string(rpc.Error))
		}
		if rpc.Result != nil {
			return rpc.Result, nil
		}
	}
	if id <= 0 {
		return nil, nil
	}
	return nil, fmt.Errorf("You.com MCP 响应里没有应答消息")
}

// youcomReadSSEMessages 收集 SSE 流里的全部 data 载荷（每条一个 JSON-RPC 消息）。
// You.com 会先推进展通知、再推应答，因此不能只取第一条；多行 data 按 SSE 规范
// 拼接。上限 maxBytes 防御异常长流。
func youcomReadSSEMessages(reader io.Reader, maxBytes int) ([]json.RawMessage, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, int64(maxBytes)))
	scanner.Buffer(make([]byte, 64*1024), maxBytes)
	var out []json.RawMessage
	var data bytes.Buffer
	flush := func() {
		if data.Len() > 0 {
			out = append(out, append([]byte(nil), data.Bytes()...))
			data.Reset()
		}
	}
	for scanner.Scan() {
		line := bytes.TrimSuffix(scanner.Bytes(), []byte("\r"))
		if len(line) == 0 {
			flush()
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.Write(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
		}
	}
	flush()
	return out, scanner.Err()
}

// youcomFirstText 取 content 里第一段非空 text。
func youcomFirstText(content []youcomContentItem) string {
	for _, c := range content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			return c.Text
		}
	}
	return ""
}

// youcomSearch 调 You.com MCP 的 you-search 工具，返回 (给模型的文本, 引用 URL 列表)。
// 未配置 YDC_API_KEY 时走免 key 的 ?profile=free 端点——选中即用，零配置。
func youcomSearch(ctx context.Context, query string, limit int) (string, []string, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	endpoint, key := youcomFreeMCPURL, youcomAPIKey()
	if key != "" {
		endpoint = youcomAuthenticatedURL
	}
	client := &http.Client{Timeout: 30 * time.Second}

	// 握手：initialize → notifications/initialized（服务端无状态，一次一发）
	initCtx, cancelInit := context.WithTimeout(ctx, 15*time.Second)
	defer cancelInit()
	if _, err := youcomMCPRequest(initCtx, client, endpoint, key, "initialize", 1, map[string]any{
		"protocolVersion": youcomProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "yosuri", "version": "0.1.0"},
	}); err != nil {
		return "", nil, err
	}
	notifyCtx, cancelNotify := context.WithTimeout(ctx, 15*time.Second)
	defer cancelNotify()
	if _, err := youcomMCPRequest(notifyCtx, client, endpoint, key, "notifications/initialized", 0, map[string]any{}); err != nil {
		return "", nil, err
	}

	callCtx, cancelCall := context.WithTimeout(ctx, 40*time.Second)
	defer cancelCall()
	raw, err := youcomMCPRequest(callCtx, client, endpoint, key, "tools/call", 2, map[string]any{
		"name":      "you-search",
		"arguments": map[string]any{"query": query, "count": limit},
	})
	if err != nil {
		return "", nil, err
	}

	// tools/call 的 result：content[0].text 里是搜索结果 JSON
	var callResult struct {
		Content []youcomContentItem `json:"content"`
		IsError bool                `json:"isError"`
	}
	if err := json.Unmarshal(raw, &callResult); err != nil {
		return "", nil, fmt.Errorf("You.com MCP 应答解析失败: %v", err)
	}
	if callResult.IsError {
		return "", nil, fmt.Errorf("You.com 搜索失败：%s", truncateChars(strings.TrimSpace(youcomFirstText(callResult.Content)), 300))
	}
	text := youcomFirstText(callResult.Content)
	if text == "" {
		return fmt.Sprintf("联网搜索「%s」没有找到结果。可以换个说法再试。", query), nil, nil
	}
	var out struct {
		Results struct {
			Web []youcomWebResult `json:"web"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return "", nil, fmt.Errorf("You.com 搜索结果解析失败: %v", err)
	}
	if len(out.Results.Web) == 0 {
		return fmt.Sprintf("联网搜索「%s」没有找到结果。可以换个说法再试。", query), nil, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "联网搜索「%s」结果（You.com）：\n", query)
	urls := make([]string, 0, len(out.Results.Web))
	for i, r := range out.Results.Web {
		if i >= limit {
			break
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n", i+1, strings.TrimSpace(r.Title), strings.TrimSpace(r.URL))
		if d := strings.TrimSpace(r.Description); d != "" {
			sb.WriteString("   " + d + "\n")
		}
		if u := strings.TrimSpace(r.URL); u != "" {
			urls = append(urls, u)
		}
	}
	return sb.String(), urls, nil
}
