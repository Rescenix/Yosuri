package main

// publish.go — 多平台一键发布（网文平台，登录 cookie）
//
// 用法：
//   rescene publish <文章.md>                 # 发布到全部已登录平台
//   rescene publish <文章.md> --platform 番茄小说
//   rescene publish --list                   # 列出平台
//
// cookie 自动获取（无需手动填）：
//   平时在 Edge 登录平台 → 发布时自动从 Edge cookie 库读取（CDP）。
//   注意：新版 Edge 运行中 cookie 库独占锁（安全设计）——发布前关掉 Edge，
//   程序自动复制 cookie 库 → headless Edge 读取 → 一键发布。
//
// 发布端点：各平台创作接口需要登录态 + 表单字段，首次发布前在配置
//   ~/rescene_data/publish_config.json 填 publish_url（登录后 F12 抓包）
//   或直接打开创作页（浏览器已登录）粘贴发布稿。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// publishConfig 发布配置（cookie 自动获取失败时的兜底 + 发布端点）
type publishConfig struct {
	Platforms map[string]pubAccount `json:"platforms"`
}

// pubAccount 单平台账号配置
type pubAccount struct {
	Cookie     string `json:"cookie,omitempty"`      // 兜底 cookie（Edge 自动获取失败时）
	PublishURL string `json:"publish_url,omitempty"` // 发布端点（登录后 F12 抓包）
	Referer    string `json:"referer,omitempty"`
}

func publishConfigPath() string {
	return filepath.Join(daughterHome(), "..", "publish_config.json")
}

func loadPublishConfig() publishConfig {
	var cfg publishConfig
	data, err := os.ReadFile(publishConfigPath())
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	if cfg.Platforms == nil {
		cfg.Platforms = map[string]pubAccount{}
	}
	return cfg
}

// publishArticle 待发布文章（从 md 文件解析：标题 = 第一个 #，正文 = 其余）
type publishArticle struct {
	Title   string
	Content string
}

func parseArticleFile(path string) (publishArticle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return publishArticle{}, err
	}
	text := strings.TrimSpace(string(data))
	lines := strings.SplitN(text, "\n", 2)
	title := strings.TrimSpace(strings.TrimPrefix(lines[0], "#"))
	title = strings.TrimSpace(strings.TrimPrefix(title, "#"))
	if title == "" {
		title = filepath.Base(path)
	}
	content := ""
	if len(lines) > 1 {
		content = strings.TrimSpace(lines[1])
	}
	return publishArticle{Title: title, Content: content}, nil
}

// runPublish 发布子命令入口
func runPublish(args []string) {
	// 参数解析
	var filePath, platformKey string
	listOnly := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--list", "-l":
			listOnly = true
		case "--platform", "-p":
			if i+1 < len(args) {
				platformKey = args[i+1]
				i++
			}
		default:
			filePath = args[i]
		}
	}

	if listOnly {
		fmt.Println("📚 支持的发布平台：")
		for _, p := range pubPlatforms {
			minLen := ""
			if p.MinLen > 0 {
				minLen = fmt.Sprintf("（需 ≥%d 字）", p.MinLen)
			}
			fmt.Printf("  %-6s %s%s — %s\n", p.ID, p.Name, minLen, p.Notes)
		}
		return
	}

	if filePath == "" {
		fmt.Println("用法: rescene publish <文章.md> [--platform 平台名] [--list]")
		return
	}

	// 解析文章
	art, err := parseArticleFile(filePath)
	if err != nil {
		fmt.Printf("❌ 读取文章失败: %v\n", err)
		return
	}
	runes := len([]rune(art.Content))
	fmt.Printf("📄 %s（%d 字）\n", art.Title, runes)

	// 目标平台
	targets := pubPlatforms
	if platformKey != "" {
		p := findPubPlatform(platformKey)
		if p == nil {
			fmt.Printf("❌ 未知平台: %s（用 --list 查看）\n", platformKey)
			return
		}
		targets = []pubPlatform{*p}
	}

	cfg := loadPublishConfig()
	ok := 0
	for _, p := range targets {
		if runes < p.MinLen {
			fmt.Printf("  ⚠️ %s 需 ≥%d 字（当前 %d）——仍尝试发布\n", p.Name, p.MinLen, runes)
		}
		if err := publishOne(p, art, cfg.Platforms[p.ID]); err != nil {
			fmt.Printf("  ❌ %s: %v\n", p.Name, err)
			continue
		}
		fmt.Printf("  ✅ %s 发布成功\n", p.Name)
		ok++
	}
	fmt.Printf("\n完成：%d/%d 平台发布成功\n", ok, len(targets))
}

// publishOne 发布到单平台：无头 Chrome 自动填表（登录态在发布专用 profile）
func publishOne(p pubPlatform, art publishArticle, acc pubAccount) error {
	// 无端点配置：无头 Chrome 自动发布（打开创作页 → 填表 → 提交）
	if acc.PublishURL == "" {
		return headlessChromePublish(p, art.Title, art.Content)
	}
	// 有端点：cookie 自动获取（Edge 调试端口或配置）+ HTTP POST
	cookie := acc.Cookie
	if cookie == "" {
		var err error
		cookie, err = edgeCookieDomain(p.Domain)
		if err != nil {
			return fmt.Errorf("自动获取 cookie 失败: %v（请先在 Edge 登录该平台）", err)
		}
	}
	if cookie == "" {
		return fmt.Errorf("未找到 %s 登录态（请先在 Edge 登录该平台）", p.Name)
	}
	return postArticleHTTP(p, acc, cookie, art)
}
