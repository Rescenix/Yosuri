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
	"net/http"
	"net/url"
	"strings"
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
	if err := cdpConn.WriteJSON(map[string]any{
		"id":     2,
		"method": "Page.startScreencast",
		"params": map[string]any{"format": "png", "everyNthFrame": 1, "quality": 80},
	}); err != nil {
		writePreviewCDPError(clientConn, "预览不可用：无法启动 Chrome 截屏")
		return
	}

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

// cdpOpenTarget 在 Chrome 里开一个新标签页并导航到 targetURL，返回该标签页的
// WebSocket 调试地址。targetURL 为空时开 about:blank。
func cdpOpenTarget(targetURL string) (tabWS string, finalURL string, err error) {
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
