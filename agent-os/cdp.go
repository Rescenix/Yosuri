package main

// cdp.go — Edge CDP cookie 自动提取（一键发布的自动化基础）
//
// 原理：用户平时在 Edge 登录了平台 → cookie 库在
//   %LOCALAPPDATA%\Microsoft\Edge\User Data\Default\Network\Cookies（SQLite，加密）
// 我们不碰加密细节：复制 cookie 库 + Local State 到临时用户目录 →
// 启动 headless Edge 加载它（Edge 自己解密）→ CDP Network.getAllCookies 取 cookie。
//
// 手写最小 WebSocket 客户端（RFC6455），纯 stdlib，无依赖。

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// edgeCookieDomain 返回某平台域名下 Edge 已登录的 cookie 字符串（"k1=v1; k2=v2"）。
// 优先连运行中的 Edge 调试端口（浏览器不关，Vue 项目不受影响）；
// 连不上 → 提示用调试模式启动 Edge（一条命令）。
func edgeCookieDomain(domain string) (string, error) {
	// 1. 连运行中的 Edge（--remote-debugging-port=9222）
	if ws := cdpRunningEdgeWS(9222); ws != "" {
		if cookies, err := cdpGetAllCookies(ws); err == nil {
			if parts := filterCookies(cookies, domain); len(parts) > 0 {
				return strings.Join(parts, "; "), nil
			}
			return "", fmt.Errorf("运行中的 Edge 未找到 %s 的登录 cookie（请先在 Edge 登录该平台）", domain)
		}
	}
	// 2. 兜底：临时 profile 复制方案（Edge 未运行/关掉时）
	return edgeCookieViaTempProfile(domain)
}

// edgeCookieViaTempProfile 复制 cookie 库 + headless 读取（Edge 关闭时可用）
func edgeCookieViaTempProfile(domain string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "rescene-edge-cookies-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)
	if err := setupTempEdgeProfile(tmpDir); err != nil {
		return "", fmt.Errorf("cookie 库被锁（Edge 正在运行）。用调试模式启动 Edge 即可自动读取，无需关闭浏览器：\n  msedge --remote-debugging-port=9222（详见 rescene edge-debug）")
	}

	port := freePort()
	edgePath := edgeExePath()
	cmd := exec.Command(edgePath,
		"--headless=new",
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--user-data-dir="+tmpDir,
		"--no-first-run", "--no-default-browser-check",
		"--disable-gpu", "--disable-extensions",
		"about:blank")
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("Edge 启动失败: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	var wsURL string
	for i := 0; i < 30; i++ {
		if wsURL = cdpPageWS(port); wsURL != "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if wsURL == "" {
		return "", fmt.Errorf("CDP 未就绪（Edge 启动异常）")
	}

	cookies, err := cdpGetAllCookies(wsURL)
	if err != nil {
		return "", err
	}
	parts := filterCookies(cookies, domain)
	if len(parts) == 0 {
		return "", fmt.Errorf("Edge 中未找到 %s 的登录 cookie（请先在 Edge 登录该平台）", domain)
	}
	return strings.Join(parts, "; "), nil
}

// cdpRunningEdgeWS 连运行中的 Edge（调试端口）返回页面 WS URL
func cdpRunningEdgeWS(port int) string {
	return cdpPageWS(port)
}

// filterCookies 按域名过滤 cookie 列表
func filterCookies(cookies []cookieRec, domain string) []string {
	var parts []string
	seen := map[string]bool{}
	for _, c := range cookies {
		if cookieMatchDomain(c.Domain, domain) && !seen[c.Name] {
			seen[c.Name] = true
			parts = append(parts, c.Name+"="+c.Value)
		}
	}
	return parts
}

// setupTempEdgeProfile 复制 cookie 库 + Local State 到临时用户目录
func setupTempEdgeProfile(tmpDir string) error {
	profile := filepath.Join(tmpDir, "Default")
	if err := os.MkdirAll(filepath.Join(profile, "Network"), 0o755); err != nil {
		return err
	}
	src := filepath.Join(edgeUserData(), "Default")
	// cookie 库
	if err := copyFile(filepath.Join(src, "Network", "Cookies"), filepath.Join(profile, "Network", "Cookies")); err != nil {
		return err
	}
	// Local State（cookie 解密 key 所在）
	copyFile(filepath.Join(edgeUserData(), "Local State"), filepath.Join(tmpDir, "Local State"))
	return nil
}

func edgeUserData() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "Microsoft", "Edge", "User Data")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", "Microsoft", "Edge", "User Data")
}

func edgeExePath() string {
	cands := []string{
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "msedge"
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// freePort 找可用端口
func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 9222
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// cdpPageWS 从 CDP HTTP 端点拿第一个页面的 WebSocket URL
func cdpPageWS(port int) string {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var pages []struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if json.NewDecoder(resp.Body).Decode(&pages) != nil || len(pages) == 0 {
		return ""
	}
	return pages[0].WebSocketDebuggerURL
}

// cookieRec CDP cookie 结构
type cookieRec struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

// cookieMatchDomain 域名匹配（.example.com 匹配 www.example.com）
func cookieMatchDomain(cookieDomain, want string) bool {
	c := strings.TrimPrefix(strings.ToLower(cookieDomain), ".")
	w := strings.ToLower(want)
	return c == w || strings.HasSuffix(c, "."+w) || strings.HasSuffix(w, "."+c)
}
