package handler

// 浏览器预览 —— 让 coding agent 写完前端文件后，其改动被后端「自动预览」逻辑
// 在「浏览器工具窗口」内嵌的真实 Chromium 里渲染并可视化。
//
// 这是内部能力，不是给模型调的工具：后端检测到 agent 改了前端文件时，自己调
// autoOpenBrowserPreview 在真实 Chromium 里开一个 target 渲染那个文件，把 target 的
// WebSocket 调试地址经 preview_open 事件回给前端。前端 PreviewBrowser 面板把地址交给
// 同源 /api/preview/cdp，由后端连接 CDP 并把 screencast 帧中转回面板——于是 agent 写的 HTML
// 不需要 iframe、也不需要桌面弹窗，直接在面板里被真实浏览器引擎渲染。
//
// 与旧的「iframe 整站首页」路线合并：不再推裸首页地址，而是直接渲染 agent 刚改的
// 那个 HTML 文件；CDP 没运行 / 改的不是 HTML / 打开失败则降级为首页 iframe。单路线，
// agent 只管写文件，后端全自动预览。

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var previewCDPUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		if strings.EqualFold(u.Host, r.Host) {
			return true
		}
		// Vite 开发代理开启 changeOrigin 后会把 Host 改成后端地址，但 Origin 仍是
		// 前端端口；两端都必须是 loopback，不能因此放开任意跨站 Origin。
		return isLoopbackHost(u.Hostname()) && isLoopbackHost(requestHostname(r))
	},
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func requestHostname(r *http.Request) string {
	u, err := url.Parse("//" + r.Host)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// validatePreviewTargetWS 只允许连接本机 Chrome 的 page target，避免把中转端点变成
// 可访问任意内网服务的 WebSocket 代理。
func validatePreviewTargetWS(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "ws" {
		return "", fmt.Errorf("CDP target 地址无效")
	}
	if !isLoopbackHost(u.Hostname()) || u.Port() != "9222" {
		return "", fmt.Errorf("仅允许连接本机 Chrome CDP(9222)")
	}
	if !strings.HasPrefix(u.Path, "/devtools/page/") {
		return "", fmt.Errorf("仅允许连接 CDP page target")
	}
	return u.String(), nil
}

func validatePreviewTargetURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Path == "" ||
		!strings.HasSuffix(strings.ToLower(u.Path), ".html") {
		return "", fmt.Errorf("预览文件地址无效")
	}
	return u.String(), nil
}

func writePreviewCDPError(conn *websocket.Conn, message string) {
	_ = conn.WriteJSON(map[string]string{"type": "error", "message": message})
}

// HandlePreviewCDP GET /api/preview/cdp?ws=<targetWS>
// 浏览器只连接这个同源端点；服务端连接真实 CDP、启动 screencast、ACK Chrome 帧，
// 并只把 PNG base64 数据转发给浏览器。
func HandlePreviewCDP(c *gin.Context) {
	var targetWS string
	var targetURL string
	var err error
	if rawWS := c.Query("ws"); rawWS != "" {
		targetWS, err = validatePreviewTargetWS(rawWS)
	} else {
		targetURL, err = validatePreviewTargetURL(c.Query("url"))
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	clientConn, err := previewCDPUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	if targetWS == "" {
		targetWS, _, err = cdpOpenTarget(targetURL)
		if err == nil {
			err = cdpNavigate(targetWS, targetURL)
		}
		if err != nil {
			writePreviewCDPError(clientConn, "预览不可用：Chrome CDP 未运行")
			return
		}
	}
	cdpConn, _, err := websocket.DefaultDialer.Dial(targetWS, nil)
	if err != nil {
		writePreviewCDPError(clientConn, "预览不可用：Chrome CDP 未运行")
		return
	}
	defer cdpConn.Close()

	if err := cdpConn.WriteJSON(map[string]any{"id": 1, "method": "Page.enable"}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启用 Chrome 页面")
		return
	}
	// 双向交互核心：启用 Input 域，这样前端的鼠标/键盘才能被打进这台 Chromium。
	if err := cdpConn.WriteJSON(map[string]any{"id": 1, "method": "Input.enable"}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启用 Chrome 输入")
		return
	}
	if err := cdpConn.WriteJSON(map[string]any{
		"id":     2,
		"method": "Page.startScreencast",
		"params": map[string]any{"format": "png", "everyNthFrame": 1, "quality": 80},
	}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启动 Chrome 截屏")
		return
	}

	// 坐标映射：前端 canvas 像素 → CDP 所需的页面坐标系。
	// 前端发 input 时带 canvas 宽高(layoutW/layoutH)与实际页面尺寸(viewW/viewH)，
	// 没有则按 1:1 透传。这样用户戳画面哪个点，就落到 Chromium 里对应的元素。
	toPageCoords := func(x, y, layoutW, layoutH, viewW, viewH float64) (float64, float64) {
		px, py := x, y
		if layoutW > 0 && viewW > 0 {
			px = x / layoutW * viewW
		}
		if layoutH > 0 && viewH > 0 {
			py = y / layoutH * viewH
		}
		return px, py
	}

	// 把前端来的 input 消息翻译成 CDP Input 命令打进 Chromium。
	dispatchInput := func(raw []byte) {
		var m struct {
			Kind string `json:"kind"` // mouse | key
			// mouse
			Action   string  `json:"action"`   // mousePressed | mouseReleased | mouseMoved
			X, Y     float64 `json:"x"`        // canvas 坐标系
			Button   string  `json:"button"`   // left | right | middle
			LayoutW  float64 `json:"layoutW"`  // 前端 canvas 显示宽度
			LayoutH  float64 `json:"layoutH"`  // 前端 canvas 显示高度
			ViewW    float64 `json:"viewW"`    // Chromium 页面宽度
			ViewH    float64 `json:"viewH"`    // Chromium 页面高度
			// key
			Key       string `json:"key"`
			Code      string `json:"code"`
			KeyAction string `json:"keyAction"` // keyDown | keyUp
		}
		if json.Unmarshal(raw, &m) != nil {
			return
		}
		switch m.Kind {
		case "mouse":
			px, py := toPageCoords(m.X, m.Y, m.LayoutW, m.LayoutH, m.ViewW, m.ViewH)
			log.Printf("🖱️ [预览输入] mouse %s @ (%.0f,%.0f) -> page(%.0f,%.0f) btn=%s",
				m.Action, m.X, m.Y, px, py, m.Button)
			cdpConn.WriteJSON(map[string]any{
				"id":     nextPreviewReqID(),
				"method": "Input.dispatchMouseEvent",
				"params": map[string]any{
					"type":       m.Action,
					"x":          px,
					"y":          py,
					"button":     m.Button,
					"clickCount": boolToInt(m.Action == "mousePressed"),
				},
			})
		case "key":
			ka := m.KeyAction
			if ka == "" {
				ka = "keyDown"
			}
			cdpConn.WriteJSON(map[string]any{
				"id":     nextPreviewReqID(),
				"method": "Input.dispatchKeyEvent",
				"params": map[string]any{
					"type":  ka,
					"key":   m.Key,
					"code":  m.Code,
					"ascii": int(m.Key[0]),
				},
			})
		}
	}

	// 两条并发读：① 从 CDP 读 screencast 帧转发给前端；② 从前端读 input 打进 CDP。
	// 用 done 通道让任一侧断开就整体退出。
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, payload, err := cdpConn.ReadMessage()
			if err != nil {
				writePreviewCDPError(clientConn, "预览不可用：Chrome CDP 连接已断开")
				return
			}
			var message struct {
				ID     int             `json:"id"`
				Error  json.RawMessage `json:"error"`
				Method string          `json:"method"`
				Params struct {
					Data      string `json:"data"`
					SessionID int    `json:"sessionId"`
				} `json:"params"`
			}
			if json.Unmarshal(payload, &message) != nil {
				continue
			}
			if message.ID == 2 && len(message.Error) > 0 {
				writePreviewCDPError(clientConn, "预览不可用：Chrome 拒绝启动截屏")
				return
			}
			if message.Method != "Page.screencastFrame" || message.Params.Data == "" {
				continue
			}
			if err := clientConn.WriteJSON(map[string]string{
				"type": "frame",
				"data": message.Params.Data,
			}); err != nil {
				return
			}
			if err := cdpConn.WriteJSON(map[string]any{
				"id":     3,
				"method": "Page.screencastFrameAck",
				"params": map[string]any{"sessionId": message.Params.SessionID},
			}); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		default:
		}
		// 读前端的消息（frame 之外唯一的交互入口）。
		mt, raw, err := clientConn.ReadMessage()
		if err != nil {
			return // 前端断开
		}
		if mt == websocket.TextMessage {
			var hdr struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &hdr) != nil {
				continue
			}
			if hdr.Type == "input" {
				dispatchInput(raw)
			}
			// 其它 type（如首帧协商）忽略，保持向后兼容。
		}
	}
}

// cdpBrowserWS 返回已运行的 Chrome 的 browser 级 WebSocket 调试地址。
// 失败返回空串（调用方据此降级为「不自动预览」）。
func cdpBrowserWS() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9222/json/version")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if json.NewDecoder(resp.Body).Decode(&v) != nil {
		return ""
	}
	return v.WebSocketDebuggerURL
}

// findChromeExecutable 定位本机 Chrome/Chromium 可执行文件。
// 优先读 CHROME_PATH 环境变量，否则在常见安装目录里找；最后回退到 PATH 查找。
// 找不到返回空串（调用方据此走降级/报错路径）。
func findChromeExecutable() string {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	candidates := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Google\Chrome Beta\Application\chrome.exe`,
		`C:\Users\` + os.Getenv("USERNAME") + `\AppData\Local\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Chromium\Application\chrome.exe`,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// ensureChromeCDP 确保本机有一个以调试模式（9222）运行的 Chrome 供预览用。
// 若 9222 已在监听则直接返回；否则自动拉起一个 headless Chrome（独立 user-data-dir，
// 不干扰用户正常使用的 Chrome）。拉起失败（找不到可执行文件/超时）则静默返回，
// 由调用方走原有降级/报错路径。这是修复「Chrome CDP 未运行」的关键：此前后端只假设
// Chrome 已以调试模式运行，从不主动拉起，导致双击打开的普通 Chrome 不满足预览条件。
func ensureChromeCDP() {
	if cdpBrowserWS() != "" {
		return // 已经在跑，别重复拉
	}
	exe := findChromeExecutable()
	if exe == "" {
		log.Printf("⚠️ [预览] 未找到 Chrome 可执行文件，无法自动拉起 CDP")
		return
	}
	userDataDir := filepath.Join(os.TempDir(), "aurora-cdp-profile")
	cmd := exec.Command(exe,
		"--headless=new",
		"--remote-debugging-port=9222",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--user-data-dir="+userDataDir,
		"about:blank",
	)
	// Windows 下隐藏可能闪现的窗口；其他平台该字段为零值不影响。
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️ [预览] 拉起 Chrome 失败: %v", err)
		return
	}
	// 轮询 9222 直到就绪（最多 ~10s），不长时间阻塞调用方。
	for i := 0; i < 40; i++ {
		time.Sleep(250 * time.Millisecond)
		if cdpBrowserWS() != "" {
			log.Printf("🖥️ [预览] 已自动拉起 Chrome CDP (pid=%d)", cmd.Process.Pid)
			return
		}
	}
	log.Printf("⚠️ [预览] Chrome 已拉起但 9222 未在超时内就绪")
}

// cdpOpenTarget 在 Chrome 里开一个新标签页并导航到 targetURL，返回该标签页的
// WebSocket 调试地址。targetURL 为空时开 about:blank。
func cdpOpenTarget(targetURL string) (tabWS string, finalURL string, err error) {
	ensureChromeCDP() // 9222 没在跑就自动拉起，修复「Chrome CDP 未运行」
	browserWS := cdpBrowserWS()
	if browserWS == "" {
		return "", "", fmt.Errorf("chrome CDP 未运行(9222 无响应)")
	}
	// 直接开带 url 的 target 最省事；CDP 的 /json/new?<url> 支持 query 形式。
	// 注意：Chrome 新版本（~109+）的 /json/new 只接受 PUT（GET/POST 返回 405），
	// 所以这里用 PUT，而不是 client.Post。
	newURL := "http://127.0.0.1:9222/json/new"
	if targetURL != "" {
		newURL += "?" + url.QueryEscape(targetURL)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, e := http.NewRequest(http.MethodPut, newURL, nil)
	if e != nil {
		return "", "", e
	}
	resp, e := client.Do(req)
	if e != nil {
		return "", "", e
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var t struct {
		ID                   string `json:"id"`
		TargetID             string `json:"targetId"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		URL                  string `json:"url"`
	}
	if json.Unmarshal(body, &t) != nil {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return "", "", fmt.Errorf("解析 /json/new 响应失败: %s", preview)
	}
	return t.WebSocketDebuggerURL, t.URL, nil
}

// cdpNavigate 连上标签页 ws，发 Page.navigate 命令导航到 targetURL（用于先开
// about:blank 再导航的场景，以及确认导航已发出）。
func cdpNavigate(tabWS, targetURL string) error {
	if tabWS == "" || targetURL == "" {
		return nil
	}
	conn, _, err := websocket.DefaultDialer.Dial(tabWS, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.WriteJSON(map[string]any{
		"id":     1,
		"method": "Page.navigate",
		"params": map[string]any{"url": targetURL},
	})
	// 等导航命令被确认即可，不必等加载完成（前端 screencast 会持续刷新）。
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, msg, e := conn.ReadMessage()
		if e != nil {
			break
		}
		var r struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(msg, &r) == nil && r.ID == 1 {
			break
		}
	}
	return nil
}

// nextPreviewReqID 返回自增的 CDP 请求 id，避免双向通道里多条命令 id 撞车。
var previewReqSeq int64

func nextPreviewReqID() int64 {
	previewReqSeq++
	return previewReqSeq
}

// boolToInt 供 CDP 鼠标事件 clickCount 字段使用（按下=1，其它=0）。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// resolvePreviewURL 把 agent 给的路径/url 规整成前端能导航的地址。
// 绝对路径 → file://；带 scheme 的 url 原样；相对路径 → 基于工作目录拼 file://。
func resolvePreviewURL(path, rawURL string) string {
	if rawURL != "" {
		if strings.Contains(rawURL, "://") {
			return rawURL
		}
		return "http://" + rawURL
	}
	if path == "" {
		return ""
	}
	p := path
	if strings.HasPrefix(p, "file://") {
		return p
	}
	// 反斜杠统一、确保 file:// 前缀
	p = strings.ReplaceAll(p, "\\", "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

// parseFrontendEditPath 从文件工具参数里取 path（前端改动检测用）。
func parseFrontendEditPath(argsJSON string) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	return a.Path, nil
}

// autoOpenBrowserPreview 自动预览核心：优先用 CDP 在真实 Chromium 里渲染 agent
// 刚改的那个 HTML 文件，返回 (url, cdpWS, "", true)。HTML 需要 CDP 但 Chrome
// 未运行/打开失败时返回可见错误；非 HTML 才继续使用前端 dev server iframe。
func autoOpenBrowserPreview(editPath string) (url string, cdpWS string, cdpError string, ok bool) {
	// 只有 HTML 文件走真实渲染才有意义（.vue/.css/.js 单文件在浏览器里看不到独立效果）。
	if editPath == "" || !strings.HasSuffix(strings.ToLower(editPath), ".html") {
		if u := aliveFrontendURL(); u != "" {
			return u, "", "", false
		}
		return "", "", "", false
	}
	target := resolvePreviewURL(editPath, "")
	if target == "" {
		return "", "", "预览不可用：无法解析预览文件地址", false
	}
	tabWS, _, err := cdpOpenTarget(target)
	if err != nil {
		return target, "", "预览不可用：Chrome CDP 未运行", false
	}
	// file:// 在 /json/new? 里有时不导航，补一次 navigate 确保生效。
	if strings.HasPrefix(target, "file://") {
		if nerr := cdpNavigate(tabWS, target); nerr != nil {
			return target, "", "预览不可用：Chrome 页面导航失败", false
		}
	}
	time.Sleep(400) // 给页面一点渲染时间
	return target, tabWS, "", true
}
