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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"backend/internal/ai/core"
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
// 预览专用端口 9223（隔离用户日常 Chrome 的 9222），同时兼容 9222 以不影响既有 MCP。
const previewCDPPort = "9223"

func validatePreviewTargetWS(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "ws" {
		return "", fmt.Errorf("CDP target 地址无效")
	}
	if !isLoopbackHost(u.Hostname()) || (u.Port() != "9222" && u.Port() != previewCDPPort) {
		return "", fmt.Errorf("仅允许连接本机 Chrome CDP(9222/9223)")
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

	// 并发写保护：screencast 帧回执 goroutine 与前端 input 分发 goroutine
	// 都会写同一个 cdpConn，gorilla/websocket 不允许并发写，否则直接 panic
	// 崩溃整个预览进程（现象：面板弹出后完全不能交互、坐标卡 0）。所有写
	// cdpConn 的路径统一走 writeCDP 加锁。
	var cdpWriteMu sync.Mutex
	writeCDP := func(v any) error {
		cdpWriteMu.Lock()
		defer cdpWriteMu.Unlock()
		return cdpConn.WriteJSON(v)
	}

	if err := writeCDP(map[string]any{"id": 1, "method": "Page.enable"}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启用 Chrome 页面")
		return
	}
	// 双向交互核心：启用 Input 域，这样前端的鼠标/键盘才能被打进这台 Chromium。
	if err := writeCDP(map[string]any{"id": 1, "method": "Input.enable"}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启用 Chrome 输入")
		return
	}
	if err := writeCDP(map[string]any{
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
		// 除零 / NaN 保护：任一缩放基准缺失或异常时，不做缩放、原样透传，
		// 避免静默落 0 误导诊断（实测坐标恒 0 多半是前端传了异常值）。
		if layoutW > 0 && viewW > 0 && !math.IsNaN(x) && !math.IsNaN(layoutW) && !math.IsNaN(viewW) {
			px = x / layoutW * viewW
		}
		if layoutH > 0 && viewH > 0 && !math.IsNaN(y) && !math.IsNaN(layoutH) && !math.IsNaN(viewH) {
			py = y / layoutH * viewH
		}
		return px, py
	}

	// 把前端来的 input 消息翻译成 CDP Input 命令打进 Chromium。
	dispatchInput := func(raw []byte) {
		var m struct {
			Kind string `json:"kind"` // mouse | key
			// mouse
			Action  string  `json:"action"`  // mousePressed | mouseReleased | mouseMoved
			X       float64 `json:"x"`       // canvas 坐标系 X（前端发 "x"）
			Y       float64 `json:"y"`       // canvas 坐标系 Y（前端发 "y"）
			Button  string  `json:"button"`  // left | right | middle
			LayoutW float64 `json:"layoutW"` // 前端 canvas 显示宽度
			LayoutH float64 `json:"layoutH"` // 前端 canvas 显示高度
			ViewW   float64 `json:"viewW"`   // Chromium 页面宽度
			ViewH   float64 `json:"viewH"`   // Chromium 页面高度
			// 诊断字段：定位 raw=(0,0) 是 clientX=0 还是 rect.left 异常
			DbgRectLeft  float64 `json:"dbgRectLeft"`
			DbgRectTop   float64 `json:"dbgRectTop"`
			DbgClientX   float64 `json:"dbgClientX"`
			DbgClientY   float64 `json:"dbgClientY"`
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
			// 诊断日志：把前端原始坐标 + rect/canvas 尺寸一起打出，
			// 便于定位「坐标恒 (0,0)」是前端 x/y 本身就是 0，还是映射后落 0。
			log.Printf("🖱️ [预览输入] mouse %s raw=(%.0f,%.0f) layout=(%.0fx%.0f) view=(%.0fx%.0f) dbg[rectL=%.0f rectT=%.0f cliX=%.0f cliY=%.0f] -> page(%.0f,%.0f) btn=%s",
				m.Action, m.X, m.Y, m.LayoutW, m.LayoutH, m.ViewW, m.ViewH,
				m.DbgRectLeft, m.DbgRectTop, m.DbgClientX, m.DbgClientY, px, py, m.Button)
			_ = writeCDP(map[string]any{
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
			_ = writeCDP(map[string]any{
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
			if err := writeCDP(map[string]any{
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

// cdpBrowserWS 返回已运行的预览专用 Chrome（端口 9223）的 browser 级 WebSocket 调试地址。
// 9223 与用户日常 Chrome 的 9222 隔离，预览永不借用/弹出用户的普通浏览器。
func cdpBrowserWS() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + previewCDPPort + "/json/version")
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

// ensureChromeCDP 确保本机有一个以调试模式（预览专用端口 9223）运行的 headless Chrome 供预览用。
// 9223 与用户日常 Chrome 的 9222 完全隔离 —— 预览永远只用这台无头实例，绝不借用/弹出
// 用户自己打开的有头 Chrome。若 9223 已在监听则直接返回；否则自动拉起一个 headless Chrome
// （独立 user-data-dir + HideWindow，Windows 下无窗口）。拉起失败则静默返回走降级/报错路径。
func ensureChromeCDP() {
	if cdpBrowserWS() != "" {
		return // 9223 已经在跑（多半是上次拉的无头实例），别重复拉
	}
	exe := findChromeExecutable()
	if exe == "" {
		log.Printf("⚠️ [预览] 未找到 Chrome 可执行文件，无法自动拉起 CDP")
		return
	}
	userDataDir := filepath.Join(os.TempDir(), "aurora-cdp-profile")
	cmd := exec.Command(exe,
		"--headless=new",
		"--remote-debugging-port="+previewCDPPort,
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
	// 轮询 9223 直到就绪（最多 ~10s），不长时间阻塞调用方。
	for i := 0; i < 40; i++ {
		time.Sleep(250 * time.Millisecond)
		if cdpBrowserWS() != "" {
			log.Printf("🖥️ [预览] 已自动拉起 headless Chrome CDP (pid=%d, port=%s)", cmd.Process.Pid, previewCDPPort)
			return
		}
	}
	log.Printf("⚠️ [预览] Chrome 已拉起但 %s 未在超时内就绪", previewCDPPort)
}

// cdpOpenTarget 在 Chrome 里开一个新标签页并导航到 targetURL，返回该标签页的
// WebSocket 调试地址。targetURL 为空时开 about:blank。
func cdpOpenTarget(targetURL string) (tabWS string, finalURL string, err error) {
	ensureChromeCDP() // 9223 没在跑就自动拉起 headless 实例，修复「Chrome CDP 未运行」
	browserWS := cdpBrowserWS()
	if browserWS == "" {
		return "", "", fmt.Errorf("chrome CDP 未运行(预览端口 " + previewCDPPort + " 无响应)")
	}
	// 注意：Chrome 新版本（~109+）的 /json/new 只接受 PUT（GET/POST 返回 405），
	// 所以这里用 PUT，而不是 client.Post。
	newURL := "http://127.0.0.1:" + previewCDPPort + "/json/new"
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

// currentPreviewTargetWS 记录「内嵌预览面板当前正在渲染的那个 CDP target」的调试地址。
// 来源：autoOpenBrowserPreview 成功开 target 后写入。LLM 的 capture_preview 工具截的就是
// 这个 target —— 即用户正在看的同一页（含用户的点击/输入状态），而不是另开一个浏览器渲染同 URL。
// 用 RWMutex 保护：预览面板每次重开都会更新它，截图工具并发读。
var (
	previewTargetMu        sync.RWMutex
	currentPreviewTargetWS string
)

// setCurrentPreviewTarget 记录当前内嵌预览 target（autoOpenBrowserPreview 成功后调用）。
func setCurrentPreviewTarget(ws string) {
	previewTargetMu.Lock()
	currentPreviewTargetWS = ws
	previewTargetMu.Unlock()
}

// getCurrentPreviewTarget 返回当前内嵌预览 target 的 ws 地址（可能为 ""）。
func getCurrentPreviewTarget() string {
	previewTargetMu.RLock()
	defer previewTargetMu.RUnlock()
	return currentPreviewTargetWS
}

// capturePreviewScreenshot 截取「内嵌预览面板当前显示的那个页面」（用户正在看的活 target）。
// url 为空 → 截 currentPreviewTargetWS（用户看到啥截到啥，含交互状态）；
// url 非空 → 复用同一台 headless Chrome（预览端口 9223）开/复用 target 截 —— 仍由 harness 控制，
// 不交给 LLM 自己开浏览器。返回的 PNG 字节可直接作为聊天图像工件发布。
func capturePreviewScreenshot(url string) ([]byte, error) {
	targetWS := ""
	if url == "" {
		targetWS = getCurrentPreviewTarget()
		if targetWS == "" {
			return nil, fmt.Errorf("当前没有正在预览的内嵌页面；可传 url 参数截指定页面")
		}
	} else {
		// 在同一台 headless Chrome 里开 target 渲染该 url（仍是 harness 的 9223 实例）。
		ws, _, err := cdpOpenTarget(url)
		if err != nil {
			return nil, fmt.Errorf("打开预览目标失败: %w", err)
		}
		if strings.HasPrefix(url, "file://") {
			if nerr := cdpNavigate(ws, url); nerr != nil {
				return nil, fmt.Errorf("预览目标导航失败: %w", nerr)
			}
		}
		// 给页面一点渲染时间
		time.Sleep(500)
		targetWS = ws
	}

	conn, _, err := websocket.DefaultDialer.Dial(targetWS, nil)
	if err != nil {
		return nil, fmt.Errorf("连接预览 target 失败: %w", err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{
		"id":     1000,
		"method": "Page.enable",
	}); err != nil {
		return nil, fmt.Errorf("启用 Page 域失败: %w", err)
	}
	// Page.captureScreenshot 截当前实时帧（含用户已做的交互），不是另起渲染。
	if err := conn.WriteJSON(map[string]any{
		"id":     1001,
		"method": "Page.captureScreenshot",
		"params": map[string]any{"format": "png", "captureBeyondViewport": false},
	}); err != nil {
		return nil, fmt.Errorf("发送截图命令失败: %w", err)
	}
	conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	for {
		_, msg, e := conn.ReadMessage()
		if e != nil {
			return nil, fmt.Errorf("读取截图结果超时: %w", e)
		}
		var r struct {
			ID     int             `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result struct {
				Data string `json:"data"`
			} `json:"result"`
		}
		if json.Unmarshal(msg, &r) != nil {
			continue
		}
		if r.ID == 1001 {
			if len(r.Error) > 0 {
				return nil, fmt.Errorf("Chrome 拒绝截图: %s", string(r.Error))
			}
			if r.Result.Data == "" {
				return nil, fmt.Errorf("截图结果为空")
			}
			return base64.StdEncoding.DecodeString(r.Result.Data)
		}
	}
}

// readCurrentPreviewText reads text from the same live CDP target shown in the
// embedded preview. It supplements a screenshot for state questions: agents
// must not infer a small score or status label from pixels when the page has
// exposed that value as readable DOM text.
func readCurrentPreviewText() (string, error) {
	targetWS := getCurrentPreviewTarget()
	if targetWS == "" {
		return "", fmt.Errorf("当前没有正在预览的内嵌页面")
	}
	conn, _, err := websocket.DefaultDialer.Dial(targetWS, nil)
	if err != nil {
		return "", fmt.Errorf("连接预览 target 失败: %w", err)
	}
	defer conn.Close()

	id := nextPreviewReqID()
	if err := conn.WriteJSON(map[string]any{
		"id": id, "method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":   "document.body ? document.body.innerText : ''",
			"returnByValue": true,
		},
	}); err != nil {
		return "", fmt.Errorf("读取预览 DOM 文本失败: %w", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	for {
		var response struct {
			ID     int64           `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result struct {
				Result struct {
					Value string `json:"value"`
				} `json:"result"`
			} `json:"result"`
		}
		if err := conn.ReadJSON(&response); err != nil {
			return "", fmt.Errorf("读取预览 DOM 文本结果失败: %w", err)
		}
		if response.ID != id {
			continue
		}
		if len(response.Error) > 0 {
			return "", fmt.Errorf("Chrome 拒绝读取预览 DOM 文本: %s", string(response.Error))
		}
		text := strings.TrimSpace(response.Result.Result.Value)
		const maxPreviewTextRunes = 4000
		runes := []rune(text)
		if len(runes) > maxPreviewTextRunes {
			text = string(runes[:maxPreviewTextRunes]) + "\n…（DOM 文本已截断）"
		}
		return text, nil
	}
}

// capturePreviewToolDef 是 harness 提供给 LLM 的「截内嵌预览页面」工具，常驻、无需 load_tools。
// 截的是用户正在看的那一页（同一 CDP target），不是另开浏览器渲染同 URL。
var capturePreviewToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "capture_preview",
		Description: "截取「内嵌浏览器预览面板当前正在显示的页面」并作为图片插入聊天。" +
			"这是用户正在看的同一页面（含用户已做的点击/输入/滚动状态），不是另开浏览器渲染的副本。" +
			"做前端开发时用来把界面效果/报错直接发到对话里。不传 url 截当前预览页；" +
			"传 url 则在同一台预览用 headless Chrome 里打开该地址再截（仍是同一个浏览器引擎）。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"url": {
					Type:        "string",
					Description: "可选。要截图的页面地址（http(s):// 或 file:// 路径）。留空则截当前内嵌预览面板正在显示的页面。",
				},
			},
			Required: []string{},
		},
	},
}

func init() {
	// 确保 core 包被引用（工具定义复用其结构，避免未使用导入告警）。
	_ = core.ToolDefinition{}
}

// extractURLArg 从 capture_preview 工具的参数 JSON 里取 url 字段（可选）。
func extractURLArg(argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	var a struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return ""
	}
	return a.URL
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
	time.Sleep(400)                // 给页面一点渲染时间
	setCurrentPreviewTarget(tabWS) // 记下活 target，供 capture_preview 截同一页
	return target, tabWS, "", true
}

// openPreviewToolDef 让 agent 主动把指定页面弹进内嵌预览面板（harness 控制的 CDP 通道，
// 不是 agent 自己开独立浏览器）。区别于系统在工作流收尾自动弹出的预览——这里 agent
// 自己决定何时展示，主动权在 agent。面板已支持双向输入（鼠标/键盘回传），故用户能直接交互/试玩。
var openPreviewToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "open_preview",
		Description: "主动把指定页面弹进内嵌浏览器预览面板，让用户可以立刻看到并交互（鼠标/键盘可操作）。" +
			"这是 agent 主动发起的预览，区别于系统在工作流收尾自动弹出的预览——你（agent）来决定何时展示，而不是等系统。" +
			"适用场景：你刚生成或修改完一个网页/游戏，想让用户马上在面板里看到并试玩时，调用本工具。" +
			"参数二选一：path 传本地 html 文件绝对路径（harness 用真实 Chromium 渲染，可交互）；" +
			"url 传 http(s) 地址（如前端 dev server 页面 http://localhost:4322）。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"path": {
					Type:        "string",
					Description: "本地 html 文件绝对路径（如 C:/Pro2026/test/pong-battle.html）。与 url 二选一。",
				},
				"url": {
					Type:        "string",
					Description: "http(s) 页面地址（如 http://localhost:4322）。与 path 二选一。",
				},
			},
			Required: []string{},
		},
	},
}

// openPreviewInPanel 把 agent 显式指定的页面弹进内嵌预览面板。
// 返回 (预览地址, CDP target ws, error)。cdpWS 非空时前端走 CDP startScreencast 真实
// 渲染（可交互）；复用 autoOpenBrowserPreview 的底层 CDP 通道，与 capture_preview /
// 系统收尾自动预览共用同一台 harness headless Chrome（端口 9223）。
func openPreviewInPanel(argsJSON string) (string, string, error) {
	var a struct {
		Path string `json:"path"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", "", fmt.Errorf("参数解析失败：path 或 url 应为字符串")
	}
	raw := a.URL
	if raw == "" {
		raw = a.Path
	}
	if raw == "" {
		return "", "", fmt.Errorf("请传入 path（本地 html 文件绝对路径）或 url（http(s) 地址）")
	}

	// http(s) url：直接在同一台 harness headless Chrome 里开 target 并导航，弹预览。
	if strings.Contains(raw, "://") {
		ws, _, err := cdpOpenTarget(raw)
		if err != nil {
			return "", "", fmt.Errorf("打开预览目标失败: %w", err)
		}
		if nerr := cdpNavigate(ws, raw); nerr != nil {
			return "", "", fmt.Errorf("预览目标导航失败: %w", nerr)
		}
		time.Sleep(400)
		setCurrentPreviewTarget(ws)
		return raw, ws, nil
	}

	// 本地路径（.html 或任意文件）：复用 autoOpenBrowserPreview 的 CDP 通道。
	url, cdpWS, cdpErr, ok := autoOpenBrowserPreview(raw)
	if !ok && cdpErr != "" {
		return "", "", fmt.Errorf("%s", cdpErr)
	}
	if url == "" {
		return "", "", fmt.Errorf("预览不可用：无法解析地址 %q", raw)
	}
	return url, cdpWS, nil
}
