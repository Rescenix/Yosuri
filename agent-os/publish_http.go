package main

// publish_http.go — 发布执行：HTTP POST（有端点时）+ 创作页兜底（无端点时）

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// postArticleHTTP 用 cookie + 发布端点 POST 文章
// 端点与表单字段：登录平台后 F12 抓发布请求填到 publish_config.json
func postArticleHTTP(p pubPlatform, acc pubAccount, cookie string, art publishArticle) error {
	form := url.Values{}
	form.Set("title", art.Title)
	form.Set("content", art.Content)
	// 常见字段名兜底（各平台不同，抓包后可调整）
	if strings.Contains(acc.PublishURL, "chapter") || strings.Contains(acc.PublishURL, "chapter") {
		form.Set("chapter_name", art.Title)
	}

	req, err := http.NewRequest("POST", acc.PublishURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookie)
	if acc.Referer != "" {
		req.Header.Set("Referer", acc.Referer)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d（可能端点/字段不对，用 F12 抓包核对 publish_url）", resp.StatusCode)
	}
	return nil
}

// openCreatePage 无发布端点时：打开平台创作页（浏览器已登录）+ 生成发布稿到 outputs/publish/
func openCreatePage(p pubPlatform, art publishArticle) error {
	// 生成发布稿（粘贴用）
	outDir := filepath.Join(daughterHome(), "outputs", "publish")
	os.MkdirAll(outDir, 0o755)
	path := filepath.Join(outDir, fmt.Sprintf("发布稿-%s-%s.md", p.ID, time.Now().Format("2006-01-02-1504")))
	content := fmt.Sprintf("# 发布稿 · %s\n\n标题：%s\n\n%s\n", p.Name, art.Title, art.Content)
	os.WriteFile(path, []byte(content), 0o644)

	// 打开创作页（默认浏览器，已登录）
	openURL(p.CreateURL)
	fmt.Printf("  已打开 %s 创作页（浏览器已登录）\n  发布稿已生成：%s\n", p.Name, path)
	return nil
}

// openURL 用默认浏览器打开链接
func openURL(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	cmd.Start()
}

// edgeDebugHint 调试模式启动 Edge 的提示（`rescene edge-debug` 用）
func edgeDebugHint() {
	fmt.Println("🔧 让 Edge 常驻调试端口（浏览器不关，cookie 自动可读）：")
	fmt.Println()
	fmt.Println("  方法 1（推荐）：命令行启动 Edge 时加参数")
	fmt.Println("    msedge --remote-debugging-port=9222")
	fmt.Println()
	fmt.Println("  方法 2：修改 Edge 快捷方式，目标末尾加：")
	fmt.Println("    --remote-debugging-port=9222")
	fmt.Println()
	fmt.Println("  设置后：rescene publish 直接连运行中的 Edge 自动读 cookie，")
	fmt.Println("  无需关闭浏览器（Vue 开发环境不受影响）。")
}
