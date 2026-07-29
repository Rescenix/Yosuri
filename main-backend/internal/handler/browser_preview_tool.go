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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"backend/internal/ai/core"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var activePreviewClientWriteMu sync.Mutex

type managedPreviewBrowser struct {
	cmd        *exec.Cmd
	port       string
	profileDir string
	done       chan struct{}
}

var (
	previewBrowserMu      sync.RWMutex
	previewBrowserStartMu sync.Mutex
	previewBrowser        *managedPreviewBrowser
)

type previewClientWriter interface {
	WriteJSON(v any) error
}

func writePreviewClient(client previewClientWriter, payload any) error {
	if client == nil {
		return fmt.Errorf("no_active_preview")
	}
	activePreviewClientWriteMu.Lock()
	defer activePreviewClientWriteMu.Unlock()
	return client.WriteJSON(payload)
}

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

// validatePreviewTargetWS 只允许连接本机 Chrome/Chromium 的 page target，避免把中转端点变成
// 可访问任意内网服务的 WebSocket 代理。
// 预览浏览器使用动态端口；9222 只用于兼容用户显式传入的既有本机 CDP target。

func validatePreviewTargetWS(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "ws" {
		return "", fmt.Errorf("CDP target 地址无效")
	}
	activePort := currentPreviewCDPPort()
	if !isLoopbackHost(u.Hostname()) || (u.Port() != "9222" && (activePort == "" || u.Port() != activePort)) {
		return "", fmt.Errorf("仅允许连接本机受管 Chromium CDP 或兼容端口 9222")
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
	_ = writePreviewClient(conn, map[string]string{"type": "error", "message": message})
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
	if err := writeCDP(map[string]any{"id": nextPreviewReqID(), "method": "Page.enable"}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启用 Chrome 页面")
		return
	}
	// 启用 Runtime 域，供前端设计 Agent 在当前页面执行通用检查或交互脚本。
	if err := writeCDP(map[string]any{"id": nextPreviewReqID(), "method": "Runtime.enable"}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启用 Chrome 运行时")
		return
	}
	// 新创建的 headless target 可能尚未处于 active 状态；Chrome 会拒绝直接
	// 启动 screencast，因此先激活页面再建立预览帧流。
	if err := writeCDP(map[string]any{"id": nextPreviewReqID(), "method": "Page.bringToFront"}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法激活 Chrome 页面")
		return
	}
	// bringToFront 的完成响应只代表命令已受理；headless target 切到 active
	// 仍需一个极短调度窗口，否则紧随其后的 startScreencast 可能被拒绝。
	time.Sleep(80 * time.Millisecond)
	startScreencastID := nextPreviewReqID()
	if err := writeCDP(map[string]any{
		"id":     startScreencastID,
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
			DbgRectLeft float64 `json:"dbgRectLeft"`
			DbgRectTop  float64 `json:"dbgRectTop"`
			DbgClientX  float64 `json:"dbgClientX"`
			DbgClientY  float64 `json:"dbgClientY"`
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
			if int64(message.ID) == startScreencastID && len(message.Error) > 0 {
				writePreviewCDPError(clientConn, "预览不可用：Chrome 拒绝启动截屏")
				return
			}
			if message.Method != "Page.screencastFrame" || message.Params.Data == "" {
				continue
			}
			if err := writePreviewClient(clientConn, map[string]string{
				"type": "frame",
				"data": message.Params.Data,
			}); err != nil {
				return
			}
			if err := writeCDP(map[string]any{
				"id":     nextPreviewReqID(),
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

// currentPreviewCDPPort 返回本进程拉起的预览浏览器端口。
// 端口由 Chromium 动态分配，不接受外部任意 loopback 端口冒充受管浏览器。
func currentPreviewCDPPort() string {
	previewBrowserMu.RLock()
	defer previewBrowserMu.RUnlock()
	if previewBrowser == nil {
		return ""
	}
	return previewBrowser.port
}

// cdpBrowserWS 返回受管预览浏览器的 browser 级 WebSocket 调试地址。
func cdpBrowserWS() string {
	return cdpBrowserWSAtPort(currentPreviewCDPPort())
}

func cdpBrowserWSAtPort(port string) string {
	if port == "" {
		return ""
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/json/version")
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

// bundledBrowserRelativePaths 是随包浏览器相对 ResceneAgent 可执行文件的目录约定。
// Chromium 是多文件运行时，必须由安装器完整复制 runtime/chromium 目录，不能只放主程序。
func bundledBrowserRelativePaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join("runtime", "chromium", "chrome.exe"),
			filepath.Join("chromium", "chrome.exe"),
		}
	case "darwin":
		return []string{
			filepath.Join("runtime", "chromium", "Chromium.app", "Contents", "MacOS", "Chromium"),
			filepath.Join("chromium", "Chromium.app", "Contents", "MacOS", "Chromium"),
		}
	default:
		return []string{
			filepath.Join("runtime", "chromium", "chrome"),
			filepath.Join("runtime", "chromium", "chromium"),
			filepath.Join("chromium", "chrome"),
		}
	}
}

func bundledBrowserCandidates(appExecutable string) []string {
	if appExecutable == "" {
		return nil
	}
	appDir := filepath.Dir(appExecutable)
	out := make([]string, 0, len(bundledBrowserRelativePaths()))
	for _, rel := range bundledBrowserRelativePaths() {
		out = append(out, filepath.Join(appDir, rel))
	}
	return out
}

func existingRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func browserInstallCandidate(base string, parts ...string) string {
	if strings.TrimSpace(base) == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, parts...)...)
}

// findChromeExecutable 定位预览专用 Chromium 内核。
// CHROME_PATH 是开发/诊断覆盖项；正式客户端优先使用 exe 旁的随包 Chromium，
// 再回退系统 Edge、Chrome/Chromium 与 PATH。
// 找不到返回空串（调用方据此走降级/报错路径）。
func findChromeExecutable() string {
	if p := strings.TrimSpace(os.Getenv("CHROME_PATH")); p != "" {
		if existingRegularFile(p) {
			return p
		}
	}

	appExecutable, _ := os.Executable()
	for _, p := range bundledBrowserCandidates(appExecutable) {
		if existingRegularFile(p) {
			return p
		}
	}

	candidates := []string{
		browserInstallCandidate(os.Getenv("PROGRAMFILES"), "Microsoft", "Edge", "Application", "msedge.exe"),
		browserInstallCandidate(os.Getenv("PROGRAMFILES(X86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		browserInstallCandidate(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Google\Chrome Beta\Application\chrome.exe`,
		browserInstallCandidate(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
		`C:\Program Files\Chromium\Application\chrome.exe`,
	}
	for _, p := range candidates {
		if p != "" && existingRegularFile(p) {
			return p
		}
	}
	for _, name := range []string{
		"msedge", "microsoft-edge", "microsoft-edge-stable",
		"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
	} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func newPreviewProfileDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDir) == "" {
		cacheDir = resceneUserDataDir()
	}
	base := filepath.Join(cacheDir, "ResceneAgent", "Chromium")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, "preview-")
}

func readDevToolsActivePort(profileDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(profileDir, "DevToolsActivePort"))
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("DevToolsActivePort 内容为空")
	}
	port := strings.TrimSpace(lines[0])
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("DevToolsActivePort 端口无效: %q", port)
	}
	return port, nil
}

func trackPreviewBrowserExit(browser *managedPreviewBrowser) {
	err := browser.cmd.Wait()
	previewBrowserMu.Lock()
	if previewBrowser == browser {
		previewBrowser = nil
	}
	previewBrowserMu.Unlock()
	close(browser.done)
	if removeErr := os.RemoveAll(browser.profileDir); removeErr != nil {
		log.Printf("⚠️ [预览] 清理 Chromium profile 失败: %v", removeErr)
	}
	if err != nil && browser.cmd.ProcessState != nil && !browser.cmd.ProcessState.Success() {
		log.Printf("ℹ️ [预览] Chromium 已退出: %v", err)
	}
}

// StopPreviewBrowser 停止本进程拉起的 Chromium/Edge 及其子进程，并清理独立 profile。
// 幂等；Wails 的 OnShutdown 与当前 server 的信号退出路径都可以直接调用。
func StopPreviewBrowser() error {
	previewBrowserMu.Lock()
	browser := previewBrowser
	previewBrowser = nil
	previewBrowserMu.Unlock()
	if browser == nil {
		return nil
	}
	err := stopPreviewProcess(browser.cmd)
	select {
	case <-browser.done:
	case <-time.After(5 * time.Second):
	}
	if removeErr := os.RemoveAll(browser.profileDir); err == nil {
		err = removeErr
	}
	return err
}

// ensureChromeCDP 确保本机有一个受管的无头 Chromium 供预览使用。
// Chromium 以 --remote-debugging-port=0 自行选择端口，写入独立 profile 的
// DevToolsActivePort；因此不会与用户浏览器或其他 Rescene 实例抢固定端口。
func ensureChromeCDP() {
	previewBrowserStartMu.Lock()
	defer previewBrowserStartMu.Unlock()

	if cdpBrowserWS() != "" {
		return
	}
	_ = StopPreviewBrowser()

	exe := findChromeExecutable()
	if exe == "" {
		log.Printf("⚠️ [预览] 未找到随包 Chromium、系统 Edge 或 Chrome，无法自动拉起 CDP")
		return
	}
	userDataDir, err := newPreviewProfileDir()
	if err != nil {
		log.Printf("⚠️ [预览] 创建 Chromium profile 失败: %v", err)
		return
	}
	cmd := exec.Command(exe,
		"--headless=new",
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=0",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--user-data-dir="+userDataDir,
		"about:blank",
	)
	configurePreviewProcess(cmd)
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(userDataDir)
		log.Printf("⚠️ [预览] 拉起 Chrome 失败: %v", err)
		return
	}

	browser := &managedPreviewBrowser{
		cmd:        cmd,
		profileDir: userDataDir,
		done:       make(chan struct{}),
	}
	previewBrowserMu.Lock()
	previewBrowser = browser
	previewBrowserMu.Unlock()
	go trackPreviewBrowserExit(browser)

	// 等 Chromium 写出动态端口并让 /json/version 就绪，最多约 10 秒。
	for i := 0; i < 40; i++ {
		time.Sleep(250 * time.Millisecond)
		port, portErr := readDevToolsActivePort(userDataDir)
		if portErr != nil {
			continue
		}
		previewBrowserMu.Lock()
		if previewBrowser == browser {
			browser.port = port
		}
		previewBrowserMu.Unlock()
		if cdpBrowserWSAtPort(port) != "" {
			log.Printf("🖥️ [预览] 已拉起受管 Chromium (pid=%d, port=%s, executable=%s)", cmd.Process.Pid, port, exe)
			return
		}
	}
	log.Printf("⚠️ [预览] Chromium 已拉起但动态 CDP 端口未在超时内就绪")
	_ = StopPreviewBrowser()
}

// cdpOpenTarget 在 Chrome 里开一个新标签页并导航到 targetURL，返回该标签页的
// WebSocket 调试地址。targetURL 为空时开 about:blank。
// cdpApplyViewport 给预览目标设置稳定的桌面视口，避免默认 800×600 导致页面
// 主体被截断。它是预览渲染适配，不依赖任何页面业务协议。
func cdpApplyViewport(tabWS string) {
	if tabWS == "" {
		return
	}
	conn, _, err := websocket.DefaultDialer.Dial(tabWS, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.WriteJSON(map[string]any{
		"id":     100,
		"method": "Emulation.setDeviceMetricsOverride",
		"params": map[string]any{
			"width":             1280,
			"height":            720,
			"deviceScaleFactor": 1,
			"mobile":            false,
		},
	})
}

func cdpOpenTarget(targetURL string) (tabWS string, finalURL string, err error) {
	ensureChromeCDP()
	port := currentPreviewCDPPort()
	browserWS := cdpBrowserWS()
	if browserWS == "" || port == "" {
		return "", "", fmt.Errorf("Chromium CDP 未运行")
	}
	// 注意：Chrome 新版本（~109+）的 /json/new 只接受 PUT（GET/POST 返回 405），
	// 所以这里用 PUT，而不是 client.Post。
	newURL := "http://127.0.0.1:" + port + "/json/new"
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
	cdpApplyViewport(t.WebSocketDebuggerURL)
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
var previewReqSeq atomic.Int64

func nextPreviewReqID() int64 {
	return previewReqSeq.Add(1)
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
// url 非空 → 复用同一台受管 headless Chromium（动态 CDP 端口）开/复用 target 截，
// 不交给 LLM 自己开浏览器。返回的 PNG 字节可直接作为聊天图像工件发布。
func capturePreviewScreenshot(url string) ([]byte, error) {
	targetWS := ""
	if url == "" {
		targetWS = getCurrentPreviewTarget()
		if targetWS == "" {
			return nil, fmt.Errorf("当前没有正在预览的内嵌页面；可传 url 参数截指定页面")
		}
	} else {
		// 在同一台受管 headless Chromium 里开 target 渲染该 URL。
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
			"expression":    "document.body ? document.body.innerText : ''",
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
		Description: "主动把指定页面弹进内嵌浏览器预览面板，让用户可以立刻看到并交互（鼠标/键盘可操作，是真实 Chromium 渲染的可交互网页，不是截图）。" +
			"这是 agent 主动发起的预览，区别于系统在工作流收尾自动弹出的预览——你（agent）来决定何时展示，而不是等系统。" +
			"适用场景：你刚生成或修改完一个网页，想让用户马上在面板里查看并交互时，调用本工具。" +
			"参数二选一：path 传本地 html 文件绝对路径（harness 用真实 Chromium 渲染，可交互）；" +
			"url 传 http(s) 地址（如前端 dev server 页面 http://localhost:4322）。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"path": {
					Type:        "string",
					Description: "本地 html 文件绝对路径（如 C:/Pro2026/test/page.html）。与 url 二选一。",
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
// 系统收尾自动预览共用同一台受管 headless Chromium（动态 CDP 端口）。
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

// evaluatePreviewExpression 在当前预览页同步执行 JS，并读取 CDP 的真实执行结果。
// 注入工具不能只看 WriteJSON 成功：那只能证明命令送出，无法证明页面没有抛错。
func evaluatePreviewExpression(expression string) (json.RawMessage, error) {
	targetWS := getCurrentPreviewTarget()
	if targetWS == "" {
		return nil, fmt.Errorf("当前没有正在预览的内嵌页面；请先 open_preview 或等系统自动弹出预览")
	}
	return evaluatePreviewExpressionAt(targetWS, expression)
}

func evaluatePreviewExpressionAt(targetWS, expression string) (json.RawMessage, error) {
	if targetWS == "" {
		return nil, fmt.Errorf("预览 target 为空")
	}
	conn, _, err := websocket.DefaultDialer.Dial(targetWS, nil)
	if err != nil {
		return nil, fmt.Errorf("连接预览 target 失败: %w", err)
	}
	defer conn.Close()

	id := nextPreviewReqID()
	if err := conn.WriteJSON(map[string]any{
		"id": id, "method": "Runtime.evaluate",
		"params": map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true},
	}); err != nil {
		return nil, fmt.Errorf("执行预览 JS 失败: %w", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	for {
		var response struct {
			ID     int64 `json:"id"`
			Result struct {
				Result struct {
					Value json.RawMessage `json:"value"`
				} `json:"result"`
				ExceptionDetails json.RawMessage `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err := conn.ReadJSON(&response); err != nil {
			return nil, fmt.Errorf("等待预览 JS 结果失败: %w", err)
		}
		if response.ID != id {
			continue
		}
		if len(response.Result.ExceptionDetails) > 0 && string(response.Result.ExceptionDetails) != "null" {
			return nil, fmt.Errorf("预览 JS 抛出异常: %s", truncateVerify(string(response.Result.ExceptionDetails)))
		}
		if len(response.Result.Result.Value) == 0 {
			return json.RawMessage("null"), nil
		}
		return response.Result.Result.Value, nil
	}
}

// injectPreviewJS 在当前预览页执行一段通用 JavaScript。它只负责网页检查、
// 交互和设计验证，不约定任何业务状态或组件协议。
func injectPreviewJS(js string) (string, error) {
	if strings.TrimSpace(js) == "" {
		return "", fmt.Errorf("js 参数不能为空")
	}
	result, err := evaluatePreviewExpression(js)
	if err != nil {
		return "", err
	}
	if string(result) == "null" {
		return "JavaScript 已在当前预览页面执行", nil
	}
	return "JavaScript 已执行，返回值：" + truncateVerify(string(result)), nil
}

// injectPreviewToolDef 让 agent 主动把 JS 注入「用户正在看的内嵌预览页面」
// （当前活跃 CDP target 的 Runtime）。区别于 open_preview（弹页面）和
// capture_preview（截图）——本工具用于前端设计检查和通用页面交互。
var injectPreviewToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "inject_preview_js",
		Description: "在用户正在看的内嵌预览页面中执行一段 JavaScript（当前活跃 CDP target 的 Runtime.evaluate）。" +
			"用于读取 DOM 状态、触发页面交互、检查响应式布局或验证前端设计结果。" +
			"页面必须已在预览面板中（先 open_preview 或等系统自动弹出）。" +
			"注入的是纯前端 JS，作用域是目标页面的 window；不要传整段 HTML。" +
			"需要读取结果时让表达式返回可 JSON 序列化的值。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"js": {
					Type:        "string",
					Description: "要执行的 JavaScript。例如：document.querySelector('button')?.click()，或 (() => ({ title: document.title }))()。",
				},
			},
			Required: []string{"js"},
		},
	},
}
