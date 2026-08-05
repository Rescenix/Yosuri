package main

// arxiv.go — arXiv 论文精读（移植自主应用 arxiv_tool.go 的检索核心）
//
// 楚门世界的「每天精读 arXiv」：抓 cs.AI/cs.LG 最新论文 → 模型挑最感兴趣的
// 一篇精读 → 写「精读笔记」进日记。
// export.arxiv.org Atom API 无密钥免费，官方限速 ~3 秒一次请求（单次调用无碍）。

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	arxivAPIEndpoint = "https://export.arxiv.org/api/query"
	arxivMaxSummary  = 700
)

// arxivPaper 单篇论文元数据
type arxivPaper struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Authors         []string `json:"authors"`
	Published       string   `json:"published"`
	PrimaryCategory string   `json:"primary_category"`
	Summary         string   `json:"summary"`
	AbsURL          string   `json:"abs_url"`
}

// ---- Atom XML 解析结构（encoding/xml 按 local name 匹配，不依赖命名空间） ----

type arxivAtomFeed struct {
	Entries []arxivAtomEntry `xml:"entry"`
}

type arxivAtomEntry struct {
	ID              string                  `xml:"id"`
	Title           string                  `xml:"title"`
	Published       string                  `xml:"published"`
	Summary         string                  `xml:"summary"`
	Authors         []arxivAtomAuthor       `xml:"author"`
	Categories      []arxivAtomCategory     `xml:"category"`
	PrimaryCategory arxivAtomPrimaryCat     `xml:"primary_category"`
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

// arxivLatest 抓最新论文（按提交时间倒序）
func arxivLatest(categories string, maxResults int) ([]arxivPaper, error) {
	var catParts []string
	for _, c := range strings.Split(categories, ",") {
		if c = strings.TrimSpace(c); c != "" {
			catParts = append(catParts, "cat:"+c)
		}
	}
	searchQuery := "all:"
	if len(catParts) > 0 {
		searchQuery = strings.Join(catParts, "+OR+")
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 20 {
		maxResults = 20
	}
	// 坑：search_query 必须字面保留 + 和 : —— url.Values.Encode() 会把 + 编码成 %2B，
	// arXiv 不认 %2B 作 AND/OR，返回空 feed。手动拼 query string。
	endpoint := fmt.Sprintf("%s?search_query=%s&sortBy=submittedDate&sortOrder=descending&max_results=%d",
		arxivAPIEndpoint, searchQuery, maxResults)

	body, err := arxivHTTPGet(context.Background(), endpoint)
	if err != nil {
		return nil, err
	}

	var feed arxivAtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("arXiv 响应解析失败：%v", err)
	}

	papers := make([]arxivPaper, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		rawID := strings.TrimSpace(e.ID)
		arxivID := rawID
		if i := strings.LastIndex(rawID, "/abs/"); i >= 0 {
			arxivID = rawID[i+len("/abs/"):]
		}
		// 去版本号（2402.03300v2 → 2402.03300）
		if i := strings.Index(arxivID, "v"); i > 0 {
			arxivID = arxivID[:i]
		}
		authors := make([]string, 0, len(e.Authors))
		for _, a := range e.Authors {
			if n := strings.TrimSpace(a.Name); n != "" {
				authors = append(authors, n)
			}
		}
		summary := strings.Join(strings.Fields(e.Summary), " ")
		if len(summary) > arxivMaxSummary {
			summary = summary[:arxivMaxSummary] + "…"
		}
		primary := strings.TrimSpace(e.PrimaryCategory.Term)
		if primary == "" && len(e.Categories) > 0 {
			primary = strings.TrimSpace(e.Categories[0].Term)
		}
		title := strings.Join(strings.Fields(e.Title), " ")
		papers = append(papers, arxivPaper{
			ID:              arxivID,
			Title:           title,
			Authors:         authors,
			Published:       strings.TrimSpace(e.Published)[:10],
			PrimaryCategory: primary,
			Summary:         summary,
			AbsURL:          "https://arxiv.org/abs/" + arxivID,
		})
	}
	return papers, nil
}

// arxivHTTPGet 抓取 arXiv API（15s 超时，带 UA；429 限速给出友好提示）
func arxivHTTPGet(ctx context.Context, endpoint string) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 arXiv 请求失败：%v", err)
	}
	req.Header.Set("User-Agent", "Rescene-AgentOS/1.0 (arxiv digest)")
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

// arxivDigest 精读一轮：抓最新论文 → 模型挑最感兴趣的 1 篇 → 写精读笔记进日记
func (d *Daughter) arxivDigest() error {
	InitRouter() // 确保模型列表已加载

	papers, err := arxivLatest("cs.AI,cs.LG", 10)
	if err != nil {
		return err
	}
	if len(papers) == 0 {
		return fmt.Errorf("arXiv 无结果")
	}

	model := pickModel(GetWorkingModels(), int(time.Now().UnixNano()))
	if model == nil {
		return fmt.Errorf("没有可用模型")
	}

	// 组装论文摘要供模型挑选
	var sb strings.Builder
	for i, p := range papers {
		fmt.Fprintf(&sb, "%d. %s（%s）\n%s\n\n", i+1, p.Title, p.PrimaryCategory, p.Summary)
	}
	prompt := fmt.Sprintf("以下是 arXiv cs.AI/cs.LG 最新论文摘要。选 1 篇你最感兴趣的，写 200-300 字「精读笔记」：为什么选它、它解决了什么问题、对你有什么启发。直接输出正文：\n\n%s", sb.String())

	msg := ChatRequest{
		Model: model.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: "你是住在一台电脑里的电子女儿，正在精读前沿论文。输出直接给正文，真实自然，不要卖萌过度。" + d.Personality.PersonalityBlock()},
			{Role: "user", Content: prompt},
		},
		Stream:      true,
		MaxTokens:   2048,
		Temperature: 0.8,
	}
	content, err := CompleteWithModel(context.Background(), model.ID, msg, func(chunk, reasoning string) {
		if !d.Silent {
			fmt.Print(chunk)
		}
	})
	if err != nil {
		return err
	}
	content = strings.TrimSpace(content)

	// 写进日记：与学习日记并列
	date := d.today()
	entry := fmt.Sprintf("\n## %s · 精读 arXiv\n\n%s\n", date, content)
	if f, err := os.OpenFile(d.Journal, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(entry)
		f.Close()
	}
	return nil
}
