package handler

// arxiv_tool.go —— arXiv 论文抓取工具（alphaXiv 风格检索/预览）。
//
// arxiv_search 由 Go 进程直接抓取 export.arxiv.org 的 Atom API（无密钥、无需
// Python/Node），解析成结构化 JSON 返回：
//   - 模型侧：拿到完整论文元数据（标题/作者/日期/分类/摘要/链接），据此总结、
//     推荐、做文献综述；
//   - 前端侧：同一份 JSON 被聊天流渲染成论文卡片预览（ArxivPaperCard），
//     标题/作者/日期/分类/摘要/PDF/HTML 按钮一应俱全，像 alphaXiv 的卡片流。
//
// 限速约定：arXiv 官方要求 ~3 秒一次请求；单次调用无碍，但模型应避免连环调用。
// 输出摘要截断到 700 字符，兼顾卡片预览与 token 成本。

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"backend/internal/ai/core"
)

// arxivToolDef arxiv_search 工具定义（随 nativeOnDemandToolDefs 按需加载）。
var arxivToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "arxiv_search",
		Description: "搜索 arXiv 论文并返回结构化元数据（标题/作者/日期/分类/摘要/PDF 链接），可用于论文检索、文献调研、追踪最新研究。按 ID 抓取、按关键词检索、按分类过滤都可以。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"query": {
					Type:        "string",
					Description: "搜索关键词（支持 arXiv 字段前缀：all:/ti:/au:/abs:/cat:，多个词用空格或 + 分隔，如 'large language models' 或 'au:hinton AND cat:cs.LG'）",
				},
				"max_results": {Type: "integer", Description: "返回数量，默认 5，最大 20"},
				"sort":        {Type: "string", Description: "排序：relevance（相关度，默认）或 submittedDate（最新提交）"},
				"categories":  {Type: "string", Description: "可选，逗号分隔的 arXiv 分类过滤，如 'cs.AI,cs.LG'"},
				"id_list":     {Type: "string", Description: "可选，按 arXiv ID 精确抓取（逗号分隔，如 '2402.03300,1706.03762'）；提供时忽略 query"},
			},
			Required: []string{},
		},
	},
}

// arxivPaper 单篇论文的结构化元数据（JSON 直出给模型与前端卡片）。
type arxivPaper struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Authors         []string `json:"authors"`
	Published       string   `json:"published"`
	Updated         string   `json:"updated"`
	PrimaryCategory string   `json:"primary_category"`
	Categories      []string `json:"categories"`
	Summary         string   `json:"summary"`
	Comment         string   `json:"comment,omitempty"`
	DOI             string   `json:"doi,omitempty"`
	AbsURL          string   `json:"abs_url"`
	PDFURL          string   `json:"pdf_url"`
	HTMLURL         string   `json:"html_url"`
}

// ---- Atom XML 解析结构（encoding/xml 按 local name 匹配，不依赖命名空间） ----

type arxivAtomFeed struct {
	Entries []arxivAtomEntry `xml:"entry"`
}

type arxivAtomEntry struct {
	ID               string                  `xml:"id"`
	Title            string                  `xml:"title"`
	Published        string                  `xml:"published"`
	Updated          string                  `xml:"updated"`
	Summary          string                  `xml:"summary"`
	Comment          string                  `xml:"comment"`
	DOI              string                  `xml:"doi"`
	Authors          []arxivAtomAuthor       `xml:"author"`
	Categories       []arxivAtomCategory     `xml:"category"`
	PrimaryCategory  arxivAtomPrimaryCat     `xml:"primary_category"`
}

type arxivAtomAuthor struct {
	Name string `xml:"name"`
}

type arxivAtomCategory struct {
	Term string `xml:"term,attr"`
}

type arxivAtomPrimaryCat struct {
	Term string `xml:"term,attr"`
}

const (
	arxivAPIEndpoint = "https://export.arxiv.org/api/query"
	arxivMaxSummary  = 700
)

// callArxivSearch 执行一次 arXiv 检索，返回 JSON 文本（模型可读、前端卡片可解析）。
func callArxivSearch(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query       string `json:"query"`
		MaxResults  int    `json:"max_results"`
		Sort        string `json:"sort"`
		Categories  string `json:"categories"`
		IDList      string `json:"id_list"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("参数解析失败：%v", err)
	}

	query := strings.TrimSpace(args.Query)
	idList := strings.TrimSpace(args.IDList)
	if query == "" && idList == "" {
		return "", fmt.Errorf("需要提供 query（搜索关键词）或 id_list（arXiv ID 列表）")
	}

	q := url.Values{}
	if idList != "" {
		q.Set("id_list", idList)
	} else {
		searchQuery := normalizeArxivQuery(query)
		// 分类过滤拼进 search_query：cat:A OR cat:B（与关键词 AND）
		if cats := strings.TrimSpace(args.Categories); cats != "" {
			var catParts []string
			for _, c := range strings.Split(cats, ",") {
				if c = strings.TrimSpace(c); c != "" {
					catParts = append(catParts, "cat:"+c)
				}
			}
			if len(catParts) > 0 {
				if searchQuery != "" {
					searchQuery += "+AND+(" + strings.Join(catParts, "+OR+") + ")"
				} else {
					searchQuery = strings.Join(catParts, "+OR+")
				}
			}
		}
		q.Set("search_query", searchQuery)
		// 排序：submittedDate 时按时间倒序（最新优先）
		if strings.EqualFold(args.Sort, "submittedDate") {
			q.Set("sortBy", "submittedDate")
			q.Set("sortOrder", "descending")
		} else {
			q.Set("sortBy", "relevance")
		}
	}
	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 20 {
		maxResults = 20
	}
	q.Set("max_results", fmt.Sprintf("%d", maxResults))

	endpoint := arxivAPIEndpoint + "?" + q.Encode()
	body, err := arxivHTTPGet(ctx, endpoint)
	if err != nil {
		return "", err
	}

	var feed arxivAtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return "", fmt.Errorf("arXiv 响应解析失败：%v", err)
	}

	papers := make([]arxivPaper, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		rawID := strings.TrimSpace(e.ID)
		// API 返回形如 http://arxiv.org/abs/2402.03300v2 → 取纯 ID（保留版本号让链接直达所读版本）
		arxivID := rawID
		if i := strings.LastIndex(rawID, "/abs/"); i >= 0 {
			arxivID = rawID[i+len("/abs/"):]
		}
		authors := make([]string, 0, len(e.Authors))
		for _, a := range e.Authors {
			if n := strings.TrimSpace(a.Name); n != "" {
				authors = append(authors, n)
			}
		}
		cats := make([]string, 0, len(e.Categories))
		for _, c := range e.Categories {
			if t := strings.TrimSpace(c.Term); t != "" {
				cats = append(cats, t)
			}
		}
		summary := strings.Join(strings.Fields(e.Summary), " ")
		if len(summary) > arxivMaxSummary {
			summary = summary[:arxivMaxSummary] + "…"
		}
		primary := strings.TrimSpace(e.PrimaryCategory.Term)
		if primary == "" && len(cats) > 0 {
			primary = cats[0]
		}
		title := strings.Join(strings.Fields(e.Title), " ")
		papers = append(papers, arxivPaper{
			ID:              arxivID,
			Title:           title,
			Authors:         authors,
			Published:       strings.TrimSpace(e.Published)[:10],
			Updated:         strings.TrimSpace(e.Updated)[:10],
			PrimaryCategory: primary,
			Categories:      cats,
			Summary:         summary,
			Comment:         strings.TrimSpace(e.Comment),
			DOI:             strings.TrimSpace(e.DOI),
			AbsURL:          "https://arxiv.org/abs/" + arxivID,
			PDFURL:          "https://arxiv.org/pdf/" + arxivID,
			HTMLURL:         "https://arxiv.org/html/" + arxivID,
		})
	}

	out := struct {
		Query  string       `json:"query"`
		Count  int          `json:"count"`
		Papers []arxivPaper `json:"papers"`
	}{
		Query:  query,
		Count:  len(papers),
		Papers: papers,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("结果序列化失败：%v", err)
	}
	return string(data), nil
}

// normalizeArxivQuery 把用户查询整理成 arXiv API 接受的 search_query：
//   - 空串 → all:（避免 400）
//   - 已有字段前缀（all:/ti:/au:/abs:/cat:/co:）则原样保留，否则包成 all:...
//   - 多个词的空格换成 +（arXiv 的空格语义 = AND）
func normalizeArxivQuery(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "all:"
	}
	if arxivHasFieldPrefix(s) {
		return s
	}
	// 布尔表达式（AND/OR/ANDNOT）原样保留，只把裸空格转 +
	return strings.ReplaceAll(s, " ", "+")
}

var arxivFieldPrefixes = []string{"all:", "ti:", "au:", "abs:", "cat:", "co:", "jr:", "rn:"}

func arxivHasFieldPrefix(s string) bool {
	lower := strings.ToLower(s)
	for _, p := range arxivFieldPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// arxivHTTPGet 抓取 arXiv API（15s 超时，带 UA；429 限速给出友好提示）。
func arxivHTTPGet(ctx context.Context, endpoint string) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 arXiv 请求失败：%v", err)
	}
	req.Header.Set("User-Agent", "Rescene-Wails/1.0 (arxiv preview plugin)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 arXiv 失败：%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("arXiv 限速中（429），请稍等几秒再试")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arXiv 返回 HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}
