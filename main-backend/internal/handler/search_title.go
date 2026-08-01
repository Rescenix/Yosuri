package handler

// 新闻标题抓取代理：DS 服务端搜索的 open_page 动作只返回 URL 不带 title，
// 前端要显示「新闻标题」就得自己抓 <title>。前端直接 fetch 新闻页会撞 CORS，
// 所以走这个后端代理（同源、无 CORS、可加 UA/超时/charset 处理）。
// 缓存：标题几乎不变，内存 map 防重复抓取。

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html/charset"
)

var (
	titleCache   = map[string]string{}
	titleCacheMu sync.Mutex
	titleRe      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// FetchPageTitle 抓取页面 <title>（3s 超时，UA 伪装，charset 自适应），失败返回 ""。
// 只认 http/https，防 SSRF。
func FetchPageTitle(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	titleCacheMu.Lock()
	if t, ok := titleCache[rawURL]; ok {
		titleCacheMu.Unlock()
		return t
	}
	titleCacheMu.Unlock()

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	// 最多读 512KB，防超大页面拖死
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return ""
	}
	title := parseHTMLTitle(body)
	if title == "" {
		return ""
	}
	titleCacheMu.Lock()
	titleCache[rawURL] = title
	titleCacheMu.Unlock()
	return title
}

// parseHTMLTitle 从 HTML 字节里提取 <title>，处理 UTF-8 / GBK 等 charset。
func parseHTMLTitle(body []byte) string {
	// 1. 先按响应声明的 charset 解码（x/net/html/charset 读 <meta>）
	enc, _, _ := charset.DetermineEncoding(body, "")
	if enc != nil {
		if s, err := enc.NewDecoder().String(string(body)); err == nil {
			body = []byte(s)
		}
	}
	m := titleRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	t := strings.TrimSpace(string(m[1]))
	t = strings.Join(strings.Fields(t), " ") // 折叠空白/换行
	if len(t) > 200 {
		t = t[:200]
	}
	return t
}

// HandleFetchTitle GET /api/fetch-title?url= → {"title": "..."}
func HandleFetchTitle(c *gin.Context) {
	raw := c.Query("url")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 url 参数"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"title": FetchPageTitle(raw)})
}
