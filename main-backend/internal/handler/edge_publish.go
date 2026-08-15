package handler

// edge_publish.go — 通过用户日常 Edge CDP 登录态自动发布（晋江/番茄）
// 不启动新浏览器，连运行中的 Edge 调试端口（9222），复用 Edge 登录态。
// 用户日常 Edge 已登录晋江/番茄 → CDP 建 tab → 导航创作页 → 填表 → 直接发表

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// edgeBrowserWS 获取用户日常 Edge 的浏览器级 WS URL（调试端口 9222）
// 区别于 cdpBrowserWS（预览浏览器端口），此函数专连用户日常 Edge
func edgeBrowserWS() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9222/json/version")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var v struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	if json.NewDecoder(resp.Body).Decode(&v) != nil {
		return ""
	}
	return v.WS
}

// cdpCreateTarget 通过浏览器级 WS 创建新 tab，返回 targetId
func cdpCreateTarget(browserWS, url string) string {
	payload, _ := json.Marshal(map[string]any{
		"id":     1,
		"method": "Target.createTarget",
		"params": map[string]any{"url": url},
	})
	resp, err := wsCall(browserWS, payload)
	if err != nil {
		return ""
	}
	var r struct {
		Result struct {
			TargetID string `json:"targetId"`
		} `json:"result"`
	}
	if json.Unmarshal(resp, &r) != nil {
		return ""
	}
	return r.Result.TargetID
}

// cdpPageWSByID 从 /json 列表找到指定 targetId 的页面 WS URL
func cdpPageWSByID(targetID string) string {
	resp, err := http.Get("http://127.0.0.1:9222/json")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var pages []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		WS   string `json:"webSocketDebuggerUrl"`
	}
	if json.NewDecoder(resp.Body).Decode(&pages) != nil {
		return ""
	}
	for _, p := range pages {
		if p.ID == targetID && p.Type == "page" {
			return p.WS
		}
	}
	return ""
}

// cdpPublishOne 通过运行中 Edge CDP 发布到单平台
// 流程：建 tab → 导航创作页 → 等待加载 → 平台特定填表提交
func cdpPublishOne(p PubPlatform, title, content string) error {
	browserWS := edgeBrowserWS()
	if browserWS == "" {
		return fmt.Errorf("Edge 调试端口 9222 未开放。请确保 Edge 以 --remote-debugging-port=9222 --remote-debugging-address=127.0.0.1 启动")
	}

	// 创建新 tab 并导航到创作页
	targetID := cdpCreateTarget(browserWS, p.CreateURL)
	if targetID == "" {
		return fmt.Errorf("创建页面失败（CDP 连接异常）")
	}

	// 等页面加载
	time.Sleep(4 * time.Second)

	// 获取页面 WS URL
	pageWS := cdpPageWSByID(targetID)
	if pageWS == "" {
		return fmt.Errorf("获取页面 WS 失败")
	}

	// 平台特定发布
	switch p.ID {
	case "jjwxc":
		return jjwxcPublish(pageWS, title, content)
	case "fanqie":
		return fanqiePublish(pageWS, title, content)
	default:
		return genericPublish(pageWS, title, content)
	}
}

// ─── 晋江发布 ───

// jjwxcPublish 晋江填表 + 直接发表（不走存稿箱）
// 坑：not_submit 会被 focus 设为 1，用 JS 设值避免触发
func jjwxcPublish(pageWS, title, content string) error {
	// 1. 设章节标题
	js := fmt.Sprintf(`document.getElementById('input_text').value = %q; document.getElementById('input_text').dispatchEvent(new Event('input',{bubbles:true}));`, title)
	cdpEval(pageWS, js)

	// 2. 设正文（name=content）
	js = fmt.Sprintf(`document.getElementById('chapterbody').value = %q; document.getElementById('chapterbody').dispatchEvent(new Event('input',{bubbles:true}));`, content)
	cdpEval(pageWS, js)

	// 3. 设 not_submit=0（可发表，非编辑中）
	cdpEval(pageWS, `document.getElementById('not_submit').value = '0';`)

	time.Sleep(500 * time.Millisecond)

	// 4. 直接发表 — 先尝试 publishnovel(0)，如果走存稿箱就点「直接发表」按钮
	r, _ := cdpEval(pageWS, `typeof publishnovel`)
	if strings.Contains(r, "function") {
		// 有 publishnovel 函数，直接调
		cdpEval(pageWS, `publishnovel(0)`)
		time.Sleep(2 * time.Second)
		// 检查是否进了存稿箱，如果是则点「直接发表」按钮
		check, _ := cdpEval(pageWS, `document.querySelector('input[value="直接发表"]') !== null ? 'has_direct_btn' : 'no_direct_btn'`)
		if strings.Contains(check, "has_direct_btn") {
			// publishnovel(0) 进了存稿箱，再点直接发表按钮
			cdpEval(pageWS, `document.querySelector('input[value="直接发表"]').click()`)
			time.Sleep(2 * time.Second)
		}
	} else {
		// 没有 publishnovel 函数，直接点按钮
		r, _ = cdpEval(pageWS, `(()=>{var b=document.querySelector('input[value="直接发表"]');if(b){b.click();return 'clicked'}return 'not_found'})()`)
		if strings.Contains(r, "not_found") {
			// 点「放入存稿箱」兜底
			cdpEval(pageWS, `document.querySelector('input[value="放入存稿箱"]')?.click()`)
		}
	}

	time.Sleep(2 * time.Second)
	return nil
}

// ─── 番茄小说发布 ───

// fanqiePublish 番茄小说自动发布
// 导航到 https://fanqienovel.com/writer → 找创作页 → 填表
func fanqiePublish(pageWS, title, content string) error {
	// 先等页面加载
	time.Sleep(3 * time.Second)

	// 检查是否已登录
	titleStr, _ := cdpEval(pageWS, `document.title`)
	if strings.Contains(titleStr, "登录") || strings.Contains(titleStr, "login") {
		return fmt.Errorf("番茄未登录，请在 Edge 中登录 https://fanqienovel.com/writer")
	}

	// 通用填表（番茄的创作页表单结构）
	cdpFillForm(pageWS, title, content)
	time.Sleep(1 * time.Second)

	// 点发布按钮
	_, err := cdpClickPublish(pageWS)
	return err
}

// ─── 通用发布 ───

// genericPublish 通用填表 + 点发布按钮（适用于未适配的平台）
func genericPublish(pageWS, title, content string) error {
	time.Sleep(3 * time.Second)
	cdpFillForm(pageWS, title, content)
	time.Sleep(1 * time.Second)
	_, err := cdpClickPublish(pageWS)
	return err
}

// ─── 辅助函数 ───

// openURL 默认浏览器打开链接
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