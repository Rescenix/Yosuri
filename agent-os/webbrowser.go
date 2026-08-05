package main

// webbrowser.go — 女儿的真浏览器上网能力（复用系统 Edge headless，免 API key）
//
// 与主应用同源哲学：不内置浏览器（发行包体积敏感），找到系统 Edge，
// 用 --headless=new --dump-dom 真实渲染页面再提取正文。
// 这是她「打开浏览器上网」的手脚——不是 API 抓取，是真浏览器在跑。
//
// 两个入口：
//   - browserSearch(query)  用搜索引擎查话题，返回结果文本（标题+摘要）
//   - edgeFetchText(url)    抓指定网页的渲染后正文

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// findEdgePath 查找系统 Edge（与主应用同款查找顺序：Program Files → x86 → LocalAppData）
func findEdgePath() string {
	candidates := []string{
		filepath.Join(os.Getenv("PROGRAMFILES"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 非 Windows 兜底：PATH 里的 edge 命令
	if p, err := exec.LookPath("microsoft-edge"); err == nil {
		return p
	}
	return ""
}

// edgeFetchText 用系统 Edge headless 真实渲染指定 URL，提取正文纯文本。
// 失败（无 Edge / 超时 / 渲染空）返回 ""，调用方自行兜底。
func edgeFetchText(target string) string {
	exe := findEdgePath()
	if exe == "" {
		return ""
	}
	// 独立 user-data-dir：Edge 单实例，用户已开着 Edge 时 --dump-dom 会被转发
	// 给现有实例然后静默退出（拿不到输出），独立目录强制新实例才可靠。
	profile, err := os.MkdirTemp("", "rescene-edge-*")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(profile)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe,
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--disable-extensions",
		"--user-data-dir="+profile,
		"--virtual-time-budget=4000",
		"--dump-dom",
		target,
	)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return domToText(string(out))
}

// browserSearch 用搜索引擎查话题（Bing 优先——国内可达；DuckDuckGo lite 兜底），
// 返回结果文本：标题 + 链接 + 摘要，格式与 firecrawlSearch 对齐，方便 LLM 消化。
func browserSearch(query string) string {
	q := url.QueryEscape(query)
	// Bing 搜索结果页：国内直连稳定，headless 渲染后正文提取
	bing := edgeFetchText("https://www.bing.com/search?q=" + q)
	if len(bing) > 200 {
		return fmt.Sprintf("话题「%s」的浏览器搜索结果（Bing）：\n%s", query, runeClip(bing, 3000))
	}
	// 兜底：DuckDuckGo lite 无 JS 纯文本结果页
	ddg := edgeFetchText("https://lite.duckduckgo.com/lite/?q=" + q)
	if len(ddg) > 200 {
		return fmt.Sprintf("话题「%s」的浏览器搜索结果（DuckDuckGo）：\n%s", query, runeClip(ddg, 3000))
	}
	return ""
}

var (
	// 注意：Go regexp(RE2) 不支持反向引用 \1，开头结尾 tag 独立匹配
	reScriptBlock = regexp.MustCompile(`(?is)<(?:script|style|noscript|svg|head|template)\b[^>]*>.*?</(?:script|style|noscript|svg|head|template)>`)
	reAnyTag      = regexp.MustCompile(`(?s)<[^>]+>`)
)

// domToText 从渲染后的 DOM 提取正文文本：剥 script/style/标签，解实体，压缩空白
func domToText(dom string) string {
	dom = reScriptBlock.ReplaceAllString(dom, " ")
	dom = reAnyTag.ReplaceAllString(dom, " ")
	dom = strings.ReplaceAll(dom, "&nbsp;", " ")
	dom = strings.ReplaceAll(dom, "&amp;", "&")
	dom = strings.ReplaceAll(dom, "&lt;", "<")
	dom = strings.ReplaceAll(dom, "&gt;", ">")
	dom = strings.ReplaceAll(dom, "&quot;", "\"")
	dom = strings.ReplaceAll(dom, "&#39;", "'")
	return strings.Join(strings.Fields(dom), " ")
}
