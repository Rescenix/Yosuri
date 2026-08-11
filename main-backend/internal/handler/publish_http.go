package handler

// publish_http.go — 发布执行：HTTP POST（有端点）+ 打开创作页兜底

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// jsonUnmarshal 容错 JSON 解析
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// guiPostArticle 用 cookie + 发布端点 POST 文章（端点/字段：登录后 F12 抓包填配置）
func guiPostArticle(acc pubAccountCfg, cookie, title, content string) error {
	form := url.Values{}
	form.Set("title", title)
	form.Set("content", content)

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
		return fmt.Errorf("HTTP %d（端点/字段可能不对，用 F12 抓包核对 publish_config.json）", resp.StatusCode)
	}
	return nil
}

// openBrowser 默认浏览器打开链接
func openBrowser(u string) {
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
