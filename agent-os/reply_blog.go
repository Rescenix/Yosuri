package main

// reply_blog.go — 博客平台适配器（CSDN 先行，架构可扩展知乎/百家号）
//
// CSDN 全站反爬：所有 API 无 Cookie 返回 JS 混淆页。必须用用户登录 Cookie
// （浏览器登录 blog.csdn.net 后复制）——这正符合"替你回复"（用自己的号）。
//
// 配置：~/rescene_data/reply_config.json
// {
//   "csdn": {
//     "cookie": "xxx",            // 浏览器登录后的 Cookie（必填）
//     "article_ids": [12345]      // 要监控评论的文章 ID（可空=拉我的最新文章）
//   }
// }
//
// 去重：已回复的评论 ID 记录在 ~/rescene_data/reply_handled.json

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// replyConfig 回复引擎配置（平台 Cookie 等）
type replyConfig struct {
	CSDN struct {
		Cookie      string `json:"cookie"`       // 浏览器登录 Cookie
		ArticleIDs  []int  `json:"article_ids"`  // 监控的文章 ID（空=自动拉我的文章）
		Username    string `json:"username"`     // CSDN 用户名（展示用）
	} `json:"csdn"`
}

func replyConfigPath() string {
	home := daughterHome()
	return filepath.Join(filepath.Dir(home), "reply_config.json")
}

// loadReplyConfig 读取配置（不存在返回空配置）
func loadReplyConfig() replyConfig {
	var cfg replyConfig
	data, err := os.ReadFile(replyConfigPath())
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	return cfg
}

// handledPath 已处理评论记录
func handledPath() string {
	home := daughterHome()
	return filepath.Join(filepath.Dir(home), "reply_handled.json")
}

// loadHandled 加载已处理评论 ID 集合
func loadHandled() map[string]bool {
	out := map[string]bool{}
	data, err := os.ReadFile(handledPath())
	if err == nil {
		var ids []string
		if json.Unmarshal(data, &ids) == nil {
			for _, id := range ids {
				out[id] = true
			}
		}
	}
	return out
}

// saveHandled 保存已处理集合（限制长度防膨胀）
func saveHandled(h map[string]bool) {
	ids := make([]string, 0, len(h))
	for id := range h {
		ids = append(ids, id)
	}
	if len(ids) > 500 {
		ids = ids[len(ids)-500:]
	}
	data, _ := json.Marshal(ids)
	os.WriteFile(handledPath(), data, 0o644)
}

// markHandledID 记录已处理评论
func markHandledID(id string) {
	h := loadHandled()
	h["csdn:"+id] = true
	saveHandled(h)
}

// isHandledID 是否已处理
func isHandledID(id string) bool {
	return loadHandled()["csdn:"+id]
}

// csdnHTTP 带 Cookie 的 CSDN 请求客户端
var csdnHTTP = &http.Client{Timeout: 20 * time.Second}

func csdnReq(method, path string, form url.Values, cookie string) ([]byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")
	req.Header.Set("Referer", "https://blog.csdn.net/")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := csdnHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if resp.StatusCode != 200 {
		return data, fmt.Errorf("CSDN HTTP %d", resp.StatusCode)
	}
	// 反爬检测：JS 混淆页特征
	if strings.Contains(string(data), "setTimeout") && strings.Contains(string(data), "0x") {
		return nil, fmt.Errorf("CSDN 反爬拦截（Cookie 失效或需重新登录）")
	}
	return data, nil
}

// csdnArticle 文章
type csdnArticle struct {
	ID    int    `json:"articleId"`
	Title string `json:"title"`
}

// csdnComment 评论
type csdnComment struct {
	ID       int    `json:"commentId"`
	Nickname string `json:"nickName"`
	Content  string `json:"commentContent"`
	Time     string `json:"commentTime"`
}

// csdnMyArticles 拉我的文章列表（带 Cookie）
func csdnMyArticles(cookie string) ([]csdnArticle, error) {
	// 我的博客列表接口
	data, err := csdnReq("GET",
		"https://blog.csdn.net/community/home-api/v1/get-business-list?page=1&size=20&businessType=blog",
		nil, cookie)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data struct {
			List []struct {
				ArticleID int    `json:"articleId"`
				Title     string `json:"title"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析文章列表失败（接口可能变动）")
	}
	var arts []csdnArticle
	for _, a := range out.Data.List {
		arts = append(arts, csdnArticle{ID: a.ArticleID, Title: a.Title})
	}
	return arts, nil
}

// csdnComments 拉文章评论（公开接口，无需登录也可读）
func csdnComments(articleID int, cookie string) ([]csdnComment, error) {
	data, err := csdnReq("GET",
		fmt.Sprintf("https://blog.csdn.net/community/home-api/v1/get-comments?articleId=%d&page=1&size=50", articleID),
		nil, cookie)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data struct {
			List []struct {
				CommentID      int    `json:"commentId"`
				NickName       string `json:"nickName"`
				CommentContent string `json:"commentContent"`
				CommentTime    string `json:"commentTime"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析评论失败（接口可能变动）")
	}
	var cmts []csdnComment
	for _, c := range out.Data.List {
		cmts = append(cmts, csdnComment{
			ID: c.CommentID, Nickname: c.NickName,
			Content: c.CommentContent, Time: c.CommentTime,
		})
	}
	return cmts, nil
}

// csdnPostComment 发表评论（带 Cookie，需要 CSRF token 从 Cookie 提取）
func csdnPostComment(articleID int, content, cookie string) error {
	// CSRF：CSDN 用 Cookie 里的 userName/Token，评论接口直接校验 Cookie
	form := url.Values{}
	form.Set("articleId", strconv.Itoa(articleID))
	form.Set("content", content)
	form.Set("parentId", "0")
	data, err := csdnReq("POST",
		"https://blog.csdn.net/community/home-api/v1/comment/add",
		form, cookie)
	if err != nil {
		return err
	}
	// 成功响应含 data.commentId
	var out struct {
		Data struct {
			CommentID int `json:"commentId"`
		} `json:"data"`
		Code int `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Errorf("解析发表结果失败: %s", truncStr(string(data), 120))
	}
	if out.Code != 0 && out.Msg != "" && !strings.Contains(string(data), "commentId") {
		return fmt.Errorf("CSDN 发表失败: %s", out.Msg)
	}
	return nil
}

func truncStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ---- 平台适配器实现 ----

// csdnAdapter CSDN 适配器
type csdnAdapter struct {
	cfg    replyConfig
	handle map[string]bool
}

func newCSDNAdapter() *csdnAdapter {
	return &csdnAdapter{cfg: loadReplyConfig(), handle: loadHandled()}
}

func (a *csdnAdapter) Name() string { return "csdn" }

// csdnCookieValid 校验 Cookie 是否像 CSDN 登录态（防假/失效 Cookie 静默返回空列表）
func csdnCookieValid(cookie string) bool {
	lower := strings.ToLower(cookie)
	return strings.Contains(lower, "sessid") || strings.Contains(lower, "username") || strings.Contains(lower, "userinfo")
}

// fetchNewComments 拉我的文章的新评论（未处理）
func (a *csdnAdapter) fetchNewComments() ([]replyEvent, error) {
	cookie := a.cfg.CSDN.Cookie
	if cookie == "" || !csdnCookieValid(cookie) {
		return nil, fmt.Errorf("未配置有效的 CSDN Cookie（浏览器登录 blog.csdn.net 后复制到 reply_config.json，需含 SESSID/userName）")
	}
	// 文章列表：配置的 ID 优先，否则自动拉我的文章
	var articles []csdnArticle
	if len(a.cfg.CSDN.ArticleIDs) > 0 {
		for _, id := range a.cfg.CSDN.ArticleIDs {
			articles = append(articles, csdnArticle{ID: id, Title: fmt.Sprintf("文章 %d", id)})
		}
	} else {
		arts, err := csdnMyArticles(cookie)
		if err != nil {
			return nil, err
		}
		articles = arts
	}

	var events []replyEvent
	for _, art := range articles {
		cmts, err := csdnComments(art.ID, cookie)
		if err != nil {
			continue // 单篇文章失败不影响整体
		}
		for _, c := range cmts {
			key := strconv.Itoa(c.ID)
			if a.handle["csdn:"+key] {
				continue
			}
			events = append(events, replyEvent{
				Platform:  "csdn",
				ID:        key,
				User:      c.Nickname,
				Text:      c.Content,
				Target:    art.Title,
				Time:      c.Time,
				ArticleID: art.ID,
			})
		}
	}
	return events, nil
}

// postReply 发表回复
func (a *csdnAdapter) postReply(ev replyEvent, reply string) error {
	if ev.ArticleID == 0 {
		return fmt.Errorf("缺少文章 ID，无法发表回复")
	}
	return csdnPostComment(ev.ArticleID, reply, a.cfg.CSDN.Cookie)
}

// markHandled 标记已处理
func (a *csdnAdapter) markHandled(ev replyEvent) {
	a.handle["csdn:"+ev.ID] = true
	saveHandled(a.handle)
}
