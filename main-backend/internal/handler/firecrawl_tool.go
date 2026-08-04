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
		Description: "联网搜索（Firecrawl）：用关键词搜索互联网，返回带标题/链接/摘要的结果列表。" +
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
		Success bool `json:"success"`
		Data    struct {
			Results []firecrawlSearchResult `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", nil, fmt.Errorf("Firecrawl 响应解析失败：%v", err)
	}
	if !out.Success {
		return "", nil, fmt.Errorf("Firecrawl 搜索失败（success=false）")
	}
	if len(out.Data.Results) == 0 {
		return fmt.Sprintf("联网搜索「%s」没有找到结果。可以换个说法再试。", query), nil, nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "联网搜索「%s」结果（Firecrawl）：\n", query)
	urls := make([]string, 0, len(out.Data.Results))
	for i, r := range out.Data.Results {
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
	text, urls, err := firecrawlSearch(ctx, query, args.MaxResults)
	if err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: text, URLs: urls}, nil
}
