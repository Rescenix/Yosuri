package handler

// firecrawl_tool.go — 联网搜索工具：Firecrawl（免费额度 500 次/月）
//
// web_search 是常驻工具（nativeWorkflowToolDefs），模型自主判断要不要联网，
// 像用 read_file 一样直接调用，无需 load_tools。
// Key 来源：前端「Firecrawl API Key」设置（user_configs id=firecrawl），
// 环境变量 FIRECRAWL_API_KEY 兜底。未配 key 时返回明确指引，不静默失败。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"backend/internal/ai/core"
)

// webSearchToolDef 联网搜索工具定义（常驻：模型第一轮就看到它，自主决定调用）
var webSearchToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "web_search",
		Description: "联网搜索：用关键词搜索互联网，返回带标题/链接/摘要的结果列表。引擎自动选择用户配置的搜索来源（默认免 key 的 Bing，或 Firecrawl/自定义模型/MCP 工具）。" +
			"当任务需要最新信息、实时数据、或你知识范围之外的网络内容时调用，也可用于核实旧知识的时效性。" +
			"搜索词用中文或英文均可。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"query":       {Type: "string", Description: "搜索关键词，一句话即可（如：2026 年诺贝尔奖得主名单）"},
				"max_results": {Type: "integer", Description: "最多返回几条结果，默认 5，最大 10"},
			},
			Required: []string{"query"},
		},
	},
}

// firecrawlAPIKey 优先用户设置（user_configs id=firecrawl，前端「Firecrawl API Key」填入），
// 环境变量 FIRECRAWL_API_KEY 兜底（CLI 场景）。
func firecrawlAPIKey() string {
	if entries, err := loadModelConfigs(""); err == nil {
		for _, e := range entries {
			if e.ID == "firecrawl" && strings.TrimSpace(e.APIKey) != "" {
				return strings.TrimSpace(e.APIKey)
			}
		}
	}
	return strings.TrimSpace(os.Getenv("FIRECRAWL_API_KEY"))
}

// firecrawlSearchResult Firecrawl /v1/search 的一条结果
type firecrawlSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// firecrawlSearch 调 Firecrawl /v1/search，返回 (给模型的文本, 引用 URL 列表)
func firecrawlSearch(ctx context.Context, query string, limit int) (string, []string, error) {
	key := firecrawlAPIKey()
	if key == "" {
		return "", nil, fmt.Errorf("未配置 Firecrawl API Key：打开设置 → 模型 → 填「Firecrawl API Key」，或设置环境变量 FIRECRAWL_API_KEY")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	body, _ := json.Marshal(map[string]any{"query": query, "limit": limit})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.firecrawl.dev/v1/search", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("Firecrawl 返回 %d：%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		Success bool                    `json:"success"`
		Data    []firecrawlSearchResult `json:"data"` // Firecrawl 实测：data 本身就是结果数组（2026-08-04）
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", nil, fmt.Errorf("Firecrawl 响应解析失败：%v", err)
	}
	if !out.Success {
		return "", nil, fmt.Errorf("Firecrawl 搜索失败（success=false）")
	}
	if len(out.Data) == 0 {
		return fmt.Sprintf("联网搜索「%s」没有找到结果。可以换个说法再试。", query), nil, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "联网搜索「%s」结果（Firecrawl）：\n", query)
	urls := make([]string, 0, len(out.Data))
	for i, r := range out.Data {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n", i+1, strings.TrimSpace(r.Title), strings.TrimSpace(r.URL))
		if d := strings.TrimSpace(r.Description); d != "" {
			sb.WriteString("   " + d + "\n")
		}
		if c := strings.TrimSpace(r.Content); c != "" {
			cc := strings.Join(strings.Fields(c), " ")
			if len(cc) > 400 {
				cc = cc[:400] + "…"
			}
			sb.WriteString("   " + cc + "\n")
		}
		if u := strings.TrimSpace(r.URL); u != "" {
			urls = append(urls, u)
		}
	}
	return sb.String(), urls, nil
}

// webSearch 按用户选的「联网来源」分发（设置面板 → 模型 → 联网来源）：
//   bing（默认）→ 免 key 的 Bing 网页搜索（国内可达，零配置）
//   firecrawl → Firecrawl /v1/search（要 key，500 次/月）
//   custom → 自定义 OpenAI 兼容 Endpoint 的 /v1/responses（内置 web_search 服务端联网）
//   mcp    → 用户指定的已装 MCP 搜索工具
// 没配置任何来源（默认 bing）时，模型也能直接联网——免 key 兜底，Rescene 原生支持联网。
func webSearch(ctx context.Context, query string, limit int) (string, []string, error) {
	entry, ok := capabilityEntry(websearchCapabilityID)
	if !ok {
		return bingSearch(ctx, query, limit)
	}
	switch capabilityMode(entry, "bing") {
	case "firecrawl":
		text, urls, err := firecrawlSearch(ctx, query, limit)
		if err != nil && firecrawlAPIKey() == "" {
			// Firecrawl 没配 key：直接降级到免 key 的 Bing，不把「请配置 key」甩给用户
			return bingSearch(ctx, query, limit)
		}
		return text, urls, err
	case "custom":
		return customModelSearch(ctx, entry, query, limit)
	case "mcp":
		return mcpToolSearch(entry, query, limit)
	default:
		return bingSearch(ctx, query, limit)
	}
}

// bingSearch 免 key 的 Bing 网页搜索（cn.bing.com，国内可达）。
// 零配置、零额度，作为联网搜索的默认兜底；解析 HTML 里的结果标题/链接/摘要。
func bingSearch(ctx context.Context, query string, limit int) (string, []string, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	u := "https://cn.bing.com/search?q=" + url.QueryEscape(query) + "&mkt=zh-CN"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Accept", "text/html")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("免 key 联网搜索失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("免 key 联网搜索 HTTP %d", resp.StatusCode)
	}

	// 解析 <li class="b_algo"> 块：<h2><a href="URL">标题</a></h2> + <p>摘要</p>
	// 提取 URL、去 HTML 标签、去 Bing 的追踪参数（?mkt=...&pc=...等）
	type item struct {
		url, title, snippet string
	}
	var items []item
	lower := string(data)
	for len(items) < limit {
		idx := strings.Index(lower, `<li class="b_algo"`)
		if idx < 0 {
			break
		}
		blockStart := idx
		blockEnd := strings.Index(lower[blockStart:], `</li>`)
		if blockEnd < 0 {
			break
		}
		block := lower[blockStart : blockStart+blockEnd]
		lower = lower[blockStart+blockEnd:]

		it := item{}
		if h2 := strings.Index(block, `<h2`); h2 >= 0 {
			if href := strings.Index(block[h2:], `href="`); href >= 0 {
				start := h2 + href + 6
				end := strings.Index(block[start:], `"`)
				if end >= 0 {
					it.url = block[start : start+end]
				}
			}
			if gt := strings.Index(block[h2:], `>`); gt >= 0 {
				titleStart := h2 + gt + 1
				if close := strings.Index(block[titleStart:], `</a>`); close >= 0 {
					it.title = stripHTMLTags(block[titleStart : titleStart+close])
				}
			}
		}
		if p := strings.Index(block, `<p`); p >= 0 {
			if gt := strings.Index(block[p:], `>`); gt >= 0 {
				snipStart := p + gt + 1
				if close := strings.Index(block[snipStart:], `</p>`); close >= 0 {
					it.snippet = stripHTMLTags(block[snipStart : snipStart+close])
				}
			}
		}
		it.url = strings.TrimSpace(it.url)
		if it.url == "" || !strings.HasPrefix(it.url, "http") {
			continue
		}
		// 去掉 Bing 的追踪参数（保留原始 URL 主体）
		if q := strings.Index(it.url, "?mkt="); q > 0 {
			it.url = it.url[:q]
		}
		if q := strings.Index(it.url, "&pc="); q > 0 {
			it.url = it.url[:q]
		}
		items = append(items, it)
	}

	if len(items) == 0 {
		return fmt.Sprintf("联网搜索「%s」没有找到结果。可以换个说法再试。", query), nil, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "联网搜索「%s」结果（Bing）：\n", query)
	urls := make([]string, 0, len(items))
	for i, r := range items {
		if i >= limit {
			break
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n", i+1, r.title, r.url)
		if s := strings.TrimSpace(r.snippet); s != "" {
			sb.WriteString("   " + s + "\n")
		}
		urls = append(urls, r.url)
	}
	return sb.String(), urls, nil
}

// stripHTMLTags 去除 HTML 标签（&lt; &gt; &amp; 等实体一并还原）。
func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	out := b.String()
	repl := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&nbsp;", " ")
	return strings.TrimSpace(repl.Replace(out))
}

// customModelSearch 调用 OpenAI 兼容 Endpoint 的 /v1/responses（带 web_search 工具），
// 服务端搜索由模型完成。DeepSeek 等服务端联网模型走这条；各家响应略有差异，宽容解析。
func customModelSearch(ctx context.Context, entry ModelConfigEntry, query string, limit int) (string, []string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(entry.Endpoint), "/")
	if endpoint == "" {
		return "", nil, fmt.Errorf("未配置联网自定义模型：打开设置 → 模型 → 联网来源 → 自定义模型，填写 Endpoint")
	}
	model := strings.TrimSpace(entry.DefaultModel)
	if model == "" {
		return "", nil, fmt.Errorf("未配置联网自定义模型名：打开设置 → 模型 → 联网来源 → 自定义模型，填写模型名")
	}
	key := strings.TrimSpace(entry.APIKey)
	base := endpoint
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": fmt.Sprintf("请联网搜索以下关键词，并把找到的结果（含标题、链接、摘要）整理成列表返回：%s", query),
		"tools": []map[string]any{{"type": "web_search"}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("联网自定义模型返回 %d：%s", resp.StatusCode, truncateChars(strings.TrimSpace(string(data)), 300))
	}
	var out struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", nil, fmt.Errorf("联网自定义模型响应解析失败：%v", err)
	}
	var sb strings.Builder
	for _, item := range out.Output {
		switch item.Type {
		case "message", "reasoning":
			for _, c := range item.Content {
				if c.Type == "output_text" || c.Type == "text" {
					if t := strings.TrimSpace(c.Text); t != "" {
						sb.WriteString(t)
						sb.WriteString("\n")
					}
				}
			}
		}
	}
	if sb.Len() == 0 {
		if t := strings.TrimSpace(out.OutputText); t != "" {
			return t, nil, nil
		}
		return fmt.Sprintf("联网搜索「%s」的模型没有返回可读结果。", query), nil, nil
	}
	return strings.TrimSpace(sb.String()), nil, nil
}

// mcpToolSearch 把 web_search 委托给用户指定的已装 MCP 工具。
func mcpToolSearch(entry ModelConfigEntry, query string, limit int) (string, []string, error) {
	tool := strings.TrimSpace(entry.Extra["mcp_tool"])
	if tool == "" {
		return "", nil, fmt.Errorf("未选择 MCP 搜索工具：打开设置 → 模型 → 联网来源 → MCP 工具")
	}
	args, _ := json.Marshal(map[string]any{"query": query, "max_results": limit})
	text, err := callMCPTool(tool, string(args))
	if err != nil {
		return "", nil, fmt.Errorf("MCP 联网搜索失败: %w", err)
	}
	return text, nil, nil
}

// callFirecrawlSearch 工具执行入口（callNativeTool 分发）
func callFirecrawlSearch(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败：%v", err)
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return nativeToolResult{}, fmt.Errorf("需要提供 query（搜索关键词）")
	}
	text, urls, err := webSearch(ctx, query, args.MaxResults)
	if err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: text, URLs: urls}, nil
}
