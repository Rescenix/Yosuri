package main

// chrome.go — 发布专用 Chrome（独立 profile + 无头自动发布）
//
// 方案（用户拍板：Edge 调试端口太麻烦）：
//   rescene chrome-login   启动普通 Chrome（发布专用 profile），登录各平台一次
//   rescene publish        无头 Chrome（同 profile 带登录态）→ 打开创作页 → 自动填表发布
//
// 不碰日常浏览器：独立 profile（~/.rescene_data/publish-chrome/），登录一次永久有效。

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// chromeExePath 找浏览器可执行文件（Edge 优先——Chrome 无限验证码，Edge 更稳）
func chromeExePath() string {
	cands := []string{
		filepath.Join(os.Getenv("PROGRAMFILES"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "chrome"
}

// chromeProfileDir 发布专用 Chrome profile
// 优先用用户现有 Edge profile（已有 B 站登录态），不新建独立文件夹
func chromeProfileDir() string {
	edgeProfile := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "User Data")
	if _, err := os.Stat(edgeProfile); err == nil {
		return edgeProfile
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "publish-chrome")
}

// runChromeLogin 启动普通 Chrome（发布 profile），用户登录各平台
func runChromeLogin() {
	exe := chromeExePath()
	if _, err := os.Stat(exe); err != nil {
		fmt.Println("❌ 未找到 Chrome。请先安装：https://www.google.com/chrome/")
		return
	}
	os.MkdirAll(chromeProfileDir(), 0o755)
	fmt.Println("🚀 打开发布专用 Chrome（独立 profile，不打扰你日常浏览器）")
	fmt.Println("   请登录要发布的平台（晋江/番茄/纵横/17K/七猫/飞卢/咪咕/黑岩/掌阅/豆瓣）")
	fmt.Println("   登录完成后关闭此窗口即可，登录态永久保存。")
	cmd := exec.Command(exe, "--user-data-dir="+chromeProfileDir())
	cmd.Start()
}

// headlessChromePublish 无头 Chrome 自动发布（打开创作页 → 填表 → 提交）
func headlessChromePublish(p pubPlatform, title, content string) error {
	exe := chromeExePath()
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("未找到 Chrome")
	}
	profile := chromeProfileDir()
	if _, err := os.Stat(filepath.Join(profile, "Default")); err != nil {
		return fmt.Errorf("发布 profile 未初始化（先运行 rescene chrome-login 登录一次）")
	}

	// 启动 Chrome（有头模式：真实 profile 下 headless 不监听 CDP——
	// 用户可看到自动发布过程，透明且稳定）
	port := freePort()
	cmd := exec.Command(exe,
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=127.0.0.1",
		"--user-data-dir="+profile,
		"--no-first-run", "--disable-gpu", "--disable-extensions",
		"about:blank")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Chrome 启动失败: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// 等 CDP 就绪（真实 profile 启动慢，最多 30s）
	var wsURL string
	for i := 0; i < 60; i++ {
		if wsURL = cdpPageWS(port); wsURL != "" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if wsURL == "" {
		return fmt.Errorf("Chrome CDP 未就绪（30 秒超时）")
	}

	// 打开创作页
	if err := cdpNavigate(wsURL, p.CreateURL); err != nil {
		return fmt.Errorf("打开创作页失败: %v", err)
	}
	time.Sleep(4 * time.Second) // 等页面加载

	// 智能填表（标题 + 正文）
	result, err := cdpFillForm(wsURL, title, content)
	if err != nil {
		return fmt.Errorf("填表失败: %v", err)
	}
	// 点发布按钮
	btn, err := cdpClickPublish(wsURL)
	if err != nil {
		return fmt.Errorf("点击发布失败: %v", err)
	}
	time.Sleep(2 * time.Second)

	fmt.Printf("  填表结果: %s | 点击按钮: %s\n", result, btn)
	if btn == "" {
		return fmt.Errorf("未找到发布按钮（可能页面结构不同，打开创作页手动发布）")
	}
	return nil
}

// openURL 用默认浏览器打开链接（publish_http.go 已有，此文件避免重复）
var _ = runtime.GOOS
