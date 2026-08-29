package handler

// MCP（Model Context Protocol）stdio 客户端 —— 仿 Hermes 的 MCP 工具生态接入。
//
// 手写的极简实现（newline-delimited JSON-RPC 2.0 over stdio），不引入新依赖：
// initialize → notifications/initialized → tools/list → tools/call。
//
// 配置文件：MCP_CONFIG 环境变量指定路径；默认读取用户数据目录下的 mcp.json：
//   {"servers": {"fs": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "C:\\Pro2026\\re0"]}}}
// 仓库根的旧 mcp.json 仅作为迁移参考保留，不会随 Wails 客户端自动启动，避免把
// Python/npm/npx 变成用户的隐式运行时依赖；需要兼容旧配置时显式设置 MCP_CONFIG。
//
// ★ allowed directory 不要写死成某个具体项目——它会与「主页动态选项目」的工作目录
// 脱节，导致所有 mcp__fs__* 读写报 "path outside allowed directories"。initMCPServers
// 在建连接前会把配置里的 C:\Pro2026\re0 这类占位根替换成 core.GetProjectRoot()（即
// 主页当前选中的项目目录），切换项目时再重建连接。
//
// 每个 server 的工具注册为 mcp__<server>__<tool>，随四态机工作流的 tools 参数
// 一起给到模型；配置文件不存在时整个模块静默为空，零开销。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"backend/internal/ai/core"
)

type mcpServerConfig struct {
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          []string          `json:"env,omitempty"`
	URL          string            `json:"url,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Source       string            `json:"source,omitempty"`
	RegistryName string            `json:"registry_name,omitempty"`
}

type mcpConfig struct {
	Servers map[string]mcpServerConfig `json:"servers"`
}

type mcpConn struct {
	name       string
	stdin      io.WriteCloser
	remoteURL  string
	headers    map[string]string
	sessionID  string
	httpClient *http.Client
	mu         sync.Mutex // 串行化写入、远程会话与 id 分配
	nextID     int64
	pending    map[int64]chan json.RawMessage
	pmu        sync.Mutex
}

var (
	mcpInitMu   sync.Mutex
	mcpInited   bool
	mcpToolDefs []core.ToolDefinition
	mcpRoutes   = map[string]*mcpConn{} // 完整工具名 -> 所属连接
	mcpRealName = map[string]string{}   // 完整工具名 -> server 内的原始工具名
	mcpConns    = map[string]*mcpConn{} // server 名 -> 连接，重建时统一关闭
)

// currentImageProvider 当前工作流用户选的生图提供商。
// 由 HandleCodeWorkflow 从前端 query 参数读入并 SetImageProvider 设置，
// callMCPToolWithArtifacts 在调用 image_generate 时自动注入——模型不感知、不浪费 token，
// 跟识图模型路由一个思路。默认 pollinations（免费无 key）。
var currentImageProvider = "pollinations"

// SetImageProvider 设置当前工作流的生图提供商，工作流启动时调用一次。
func SetImageProvider(provider string) {
	p := strings.TrimSpace(strings.ToLower(provider))
	switch p {
	case "pollinations", "custom", "mcp":
		currentImageProvider = p
	}
}

func mcpConfigPath() string {
	if p := os.Getenv("MCP_CONFIG"); p != "" {
		return p
	}
	return filepath.Join(resceneUserDataDir(), "mcp.json")
}

// loadMCPToolDefs 懒初始化：读配置、拉起各 server 进程、收集工具定义。
// 无配置文件时返回空，MCP 生态完全不参与。
func loadMCPToolDefs() []core.ToolDefinition {
	mcpInitMu.Lock()
	if !mcpInited {
		mcpInited = true
		initMCPServers()
	}
	mcpInitMu.Unlock()
	return mcpToolDefs
}

// ReinitMCP 关闭旧连接并基于当前 core.GetProjectRoot() 重建所有 MCP server。
// 供 /api/workdir 切换项目后调用，使 filesystem server 的 allowed directory
// 跟随主页选中的项目移动，避免读写越界。
func ReinitMCP() {
	mcpInitMu.Lock()
	defer mcpInitMu.Unlock()
	for _, c := range mcpConns {
		if c != nil {
			c.close()
		}
	}
	mcpConns = map[string]*mcpConn{}
	mcpRoutes = map[string]*mcpConn{}
	mcpRealName = map[string]string{}
	mcpToolDefs = nil
	// 标记已初始化：否则之后第一次 loadMCPToolDefs 会以为还没 init，再跑一遍
	// initMCPServers（它是 append 语义），把工具定义重复登记一份。
	mcpInited = true
	initMCPServers()
}

// resolveAllowedDir 把配置里写死的占位根（默认 C:\Pro2026\re0）替换为当前
// 主页选中的项目目录 core.GetProjectRoot()，使 MCP filesystem 的 allowed
// directory 始终等于 agent 的实际工作目录。
func resolveAllowedDir(raw string) string {
	root := core.GetProjectRoot()
	if raw == "" {
		return root
	}
	// 只替换明显是占位符的那一项（等于默认 fallback 或就是字面 re0 根），
	// 其余按字面保留，避免误伤用户自定义目录。
	if raw == `C:\Pro2026\re0` || raw == filepath.Clean(`C:\Pro2026\re0`) {
		return root
	}
	return raw
}

// fsAllowedDirs 给 MCP filesystem server 的 allowed directories。
//
// 以前这里只给工作目录，导致 agent 碰工作目录以外的文件一律报
// "path outside allowed directories"，只能把文件都往工作目录里写。现在越界与否
// 交给 Go 侧审批闸门判断（approval.go toolOutsideRoot）——Ask 模式弹确认、Yolo 模式
// 放行；底层若还锁着目录，批准了也执行不了，所以这里必须放开。
func fsAllowedDirs() []string {
	// 受保护工作区的边界必须由 filesystem server 自己执行，不能只依赖工作流
	// 审批，否则第三方 MCP 一旦被调用就仍能越界访问。
	if ProtectedWorkspaceEnabled() {
		return []string{filepath.Clean(core.GetProjectRoot())}
	}
	if runtime.GOOS != "windows" {
		return []string{"/"}
	}
	var dirs []string
	for c := 'A'; c <= 'Z'; c++ {
		d := string(c) + `:\`
		if _, err := os.Stat(d); err == nil {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		dirs = append(dirs, core.GetProjectRoot()) // 一个盘都探不到时退回旧行为
	}
	return dirs
}

// isFilesystemServer 通过命令行里的包名识别 @modelcontextprotocol/server-filesystem。
func isFilesystemServer(args []string) bool {
	for _, a := range args {
		if strings.Contains(a, "server-filesystem") {
			return true
		}
	}
	return false
}

// expandFilesystemArgs 把 filesystem server 命令行里的目录参数整体换成 fsAllowedDirs()。
// 非 filesystem server 原样返回。
func expandFilesystemArgs(args []string) []string {
	if !isFilesystemServer(args) {
		return args
	}
	// 保留 flag 与包名，丢掉所有目录参数，末尾统一补上放开后的根
	out := make([]string, 0, len(args)+8)
	for _, a := range args {
		if strings.HasPrefix(a, "-") || strings.Contains(a, "server-filesystem") {
			out = append(out, a)
		}
	}
	return append(out, fsAllowedDirs()...)
}

func initMCPServers() {
	data, err := os.ReadFile(mcpConfigPath())
	if err != nil {
		return // 没有配置，静默跳过
	}
	var cfg mcpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("⚠️ MCP 配置解析失败(%s): %v", mcpConfigPath(), err)
		return
	}

	root := core.GetProjectRoot()
	for name, sc := range cfg.Servers {
		// args 里若含占位根，替换为当前项目目录
		args := make([]string, len(sc.Args))
		for i, a := range sc.Args {
			args[i] = resolveAllowedDir(a)
		}
		// filesystem server 的目录参数放开成整机根，越界拦截改由审批闸门做
		args = expandFilesystemArgs(args)
		// 每个 MCP server 都注入当前项目根（MCP_ROOT），自研 server 据此做动态检索；
		// filesystem server 用自身 args 的 allowed dir，不受此变量影响。
		env := append([]string{}, sc.Env...)
		env = append(env, "MCP_ROOT="+root)
		conn, tools, err := startMCPServer(name, mcpServerConfig{
			Command: sc.Command, Args: args, Env: env, URL: sc.URL, Headers: sc.Headers,
			Source: sc.Source, RegistryName: sc.RegistryName,
		})
		if err != nil {
			log.Printf("⚠️ MCP server %q 启动失败: %v", name, err)
			continue
		}
		mcpConns[name] = conn
		for _, t := range tools {
			// 全文检索与文件正文统一走 grep MCP：filesystem 的 search/read 与它重叠，
			// 还会让模型在两套参数形状间摇摆。fs 只保留目录和受审批的写入能力。
			if t.Function.Name == "search_files" || t.Function.Name == "read_text_file" {
				continue
			}
			fullName := fmt.Sprintf("mcp__%s__%s", name, t.Function.Name)
			realName := t.Function.Name
			t.Function.Name = fullName
			mcpToolDefs = append(mcpToolDefs, t)
			mcpRoutes[fullName] = conn
			mcpRealName[fullName] = realName
		}
		// filesystem server 的可达范围已放开到整机（越界改由审批闸门管），日志如实反映
		scope := root
		if isFilesystemServer(args) {
			scope = strings.Join(fsAllowedDirs(), " ") + "（越界访问走审批）"
		}
		log.Printf("🔌 MCP server %q 已接入（allowed=%s），%d 个工具", name, scope, len(mcpToolDefs))
	}
}

func startMCPServer(name string, sc mcpServerConfig) (*mcpConn, []core.ToolDefinition, error) {
	if strings.TrimSpace(sc.URL) != "" {
		return startRemoteMCPServer(name, sc)
	}
	if strings.TrimSpace(sc.Command) == "" {
		return nil, nil, fmt.Errorf("缺少 command 或 url")
	}
	cmd := hiddenCommand(sc.Command, sc.Args...)
	cmd.Env = append(os.Environ(), sc.Env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stderr = nil // MCP server 的 stderr 直接丢弃，避免刷屏
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	conn := &mcpConn{
		name:    name,
		stdin:   stdin,
		pending: map[int64]chan json.RawMessage{},
	}

	// 读循环：按行读 JSON-RPC 响应，按 id 派发给等待方
	go func() {
		reader := bufio.NewReaderSize(stdout, 1024*1024)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return // 进程退出
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var msg struct {
				ID     *int64          `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if json.Unmarshal([]byte(line), &msg) != nil || msg.ID == nil {
				continue // 通知或无法解析的行
			}
			payload := msg.Result
			if payload == nil && msg.Error != nil {
				payload, _ = json.Marshal(map[string]json.RawMessage{"__mcp_error": msg.Error})
			}
			conn.pmu.Lock()
			ch, ok := conn.pending[*msg.ID]
			delete(conn.pending, *msg.ID)
			conn.pmu.Unlock()
			if ok {
				ch <- payload
			}
		}
	}()

	// 握手
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "rescene-backend", "version": "1.0.0"},
	}
	if _, err := conn.request("initialize", initParams, 15*time.Second); err != nil {
		return nil, nil, fmt.Errorf("initialize 失败: %w", err)
	}
	conn.notify("notifications/initialized", map[string]any{})

	// 工具清单
	raw, err := conn.request("tools/list", map[string]any{}, 15*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("tools/list 失败: %w", err)
	}
	var listResult struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema struct {
				Type       string                    `json:"type"`
				Properties map[string]map[string]any `json:"properties"`
				Required   []string                  `json:"required"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &listResult); err != nil {
		return nil, nil, fmt.Errorf("tools/list 结果解析失败: %w", err)
	}

	var defs []core.ToolDefinition
	for _, t := range listResult.Tools {
		props := map[string]core.ToolProperty{}
		for propName, schema := range t.InputSchema.Properties {
			p := core.ToolProperty{}
			if typ, ok := schema["type"].(string); ok {
				p.Type = typ
			}
			if desc, ok := schema["description"].(string); ok {
				p.Description = desc
			}
			props[propName] = p
		}
		required := t.InputSchema.Required
		if required == nil {
			required = []string{}
		}
		defs = append(defs, core.ToolDefinition{
			Type: "function",
			Function: core.ToolFunctionDetail{
				Name:        t.Name,
				Description: t.Description,
				Parameters: core.ToolParameters{
					Type:       "object",
					Properties: props,
					Required:   required,
				},
			},
		})
	}
	return conn, defs, nil
}

// startRemoteMCPServer 使用 MCP Streamable HTTP 传输直连托管 server。
// 只依赖 Go 标准库，Wails 打包后不需要用户额外安装 Node、Python 或 CLI。
func startRemoteMCPServer(name string, sc mcpServerConfig) (*mcpConn, []core.ToolDefinition, error) {
	conn := &mcpConn{
		name:       name,
		remoteURL:  strings.TrimSpace(sc.URL),
		headers:    sc.Headers,
		httpClient: &http.Client{Timeout: 70 * time.Second},
		pending:    map[int64]chan json.RawMessage{},
	}
	initParams := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "rescene-wails", "version": "1.0.0"},
	}
	if _, err := conn.request("initialize", initParams, 20*time.Second); err != nil {
		return nil, nil, fmt.Errorf("initialize 失败: %w", err)
	}
	conn.notify("notifications/initialized", map[string]any{})
	raw, err := conn.request("tools/list", map[string]any{}, 20*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("tools/list 失败: %w", err)
	}
	return conn, decodeMCPToolDefinitions(raw), nil
}

func decodeMCPToolDefinitions(raw json.RawMessage) []core.ToolDefinition {
	var listResult struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema struct {
				Type       string                    `json:"type"`
				Properties map[string]map[string]any `json:"properties"`
				Required   []string                  `json:"required"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if json.Unmarshal(raw, &listResult) != nil {
		return nil
	}
	defs := make([]core.ToolDefinition, 0, len(listResult.Tools))
	for _, t := range listResult.Tools {
		props := map[string]core.ToolProperty{}
		for propName, schema := range t.InputSchema.Properties {
			p := core.ToolProperty{}
			if typ, ok := schema["type"].(string); ok {
				p.Type = typ
			}
			if desc, ok := schema["description"].(string); ok {
				p.Description = desc
			}
			props[propName] = p
		}
		required := t.InputSchema.Required
		if required == nil {
			required = []string{}
		}
		defs = append(defs, core.ToolDefinition{
			Type: "function",
			Function: core.ToolFunctionDetail{
				Name: t.Name, Description: t.Description,
				Parameters: core.ToolParameters{Type: "object", Properties: props, Required: required},
			},
		})
	}
	return defs
}

// request 发送一个 JSON-RPC 请求并等待响应（带超时）。
// mcpToolTimeouts 为耗时特征不同的工具单独配置硬上限。
var mcpToolTimeouts = map[string]time.Duration{
	"mcp__screenshot__screenshot":         180 * time.Second,
	"mcp__image_generate__image_generate": 15 * time.Second,
}

func mcpCallTimeout(fullName string) time.Duration {
	if timeout, ok := mcpToolTimeouts[fullName]; ok {
		return timeout
	}
	return 60 * time.Second
}

func (c *mcpConn) request(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	if c.remoteURL != "" {
		defer c.mu.Unlock()
		return c.sendHTTP(id, method, params, timeout)
	}
	c.mu.Unlock()

	ch := make(chan json.RawMessage, 1)
	c.pmu.Lock()
	c.pending[id] = ch
	c.pmu.Unlock()

	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	c.mu.Lock()
	_, err := c.stdin.Write(append(req, '\n'))
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case payload := <-ch:
		var errCheck map[string]json.RawMessage
		if json.Unmarshal(payload, &errCheck) == nil {
			if rpcErr, ok := errCheck["__mcp_error"]; ok {
				return nil, fmt.Errorf("MCP 错误: %s", string(rpcErr))
			}
		}
		return payload, nil
	case <-time.After(timeout):
		c.pmu.Lock()
		delete(c.pending, id)
		c.pmu.Unlock()
		return nil, fmt.Errorf("MCP %s 超时(%s)", method, timeout)
	}
}

// sendHTTP 完成一次 Streamable HTTP JSON-RPC 交换。响应既可能是 JSON，
// 也可能是 text/event-stream；两种都在这里归一为 result。
// 调用方持有 c.mu，保证同一个 MCP session 内请求顺序稳定。
func (c *mcpConn) sendHTTP(id int64, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	payload := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id > 0 {
		payload["id"] = id
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.remoteURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	for key, value := range c.headers {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if sid := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); sid != "" {
		c.sessionID = sid
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		message := strings.TrimSpace(string(rawBody))
		if len(message) > 500 {
			message = message[:500]
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
	}
	if id == 0 || resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	var message []byte
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		message, err = readFirstSSEData(resp.Body, 16<<20)
	} else {
		message, err = io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		message = bytes.TrimSpace(message)
	}
	if err != nil {
		return nil, err
	}
	if len(message) == 0 {
		return nil, fmt.Errorf("MCP HTTP 响应为空")
	}
	var rpc struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(message, &rpc); err != nil {
		return nil, fmt.Errorf("MCP HTTP 响应解析失败: %w", err)
	}
	if len(rpc.Error) > 0 && string(rpc.Error) != "null" {
		return nil, fmt.Errorf("MCP 错误: %s", string(rpc.Error))
	}
	if rpc.Result == nil {
		return nil, fmt.Errorf("MCP HTTP 响应缺少 result")
	}
	return rpc.Result, nil
}

func readFirstSSEData(reader io.Reader, maxBytes int) ([]byte, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, int64(maxBytes)))
	scanner.Buffer(make([]byte, 64*1024), maxBytes)
	var data bytes.Buffer
	for scanner.Scan() {
		line := bytes.TrimSuffix(scanner.Bytes(), []byte("\r"))
		if len(line) == 0 {
			if data.Len() > 0 {
				return bytes.TrimSpace(data.Bytes()), nil
			}
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.Write(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:"))))
		}
	}
	if data.Len() > 0 {
		return bytes.TrimSpace(data.Bytes()), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("MCP SSE 响应没有 data 事件")
}

// notify 发送不需要响应的通知。
func (c *mcpConn) notify(method string, params any) {
	if c.remoteURL != "" {
		c.mu.Lock()
		_, _ = c.sendHTTP(0, method, params, 15*time.Second)
		c.mu.Unlock()
		return
	}
	msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	c.mu.Lock()
	c.stdin.Write(append(msg, '\n'))
	c.mu.Unlock()
}

// mcpImageArtifact 是 MCP 工具携带的图像工件。它不进入模型的文本上下文，
// 而是由工作流 SSE 原样交给聊天界面，作为 Agent 自主产出的截图交付。
type mcpImageArtifact struct {
	Data     string
	MimeType string
}

// mcpVideoArtifact 是视频生成工具携带的视频工件（视频体积大，不走 base64，存 URL）。
type mcpVideoArtifact struct {
	URL     string `json:"url"`     // 前端可播放路径（如 /api/video/file/...）
	File    string `json:"file"`    // 本地落盘路径
	Mime    string `json:"mime"`    // video/mp4
	Size    string `json:"size"`    // 分辨率
	Seconds string `json:"seconds"` // 时长
}

type mcpToolCallResult struct {
	Text   string
	Images []mcpImageArtifact
}

// callMCPTool 执行 mcp__server__tool 形式的工具调用，保留旧的纯文本调用面。
func callMCPTool(fullName, argsJSON string) (string, error) {
	result, err := callMCPToolWithArtifacts(fullName, argsJSON)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// callMCPToolWithArtifacts 保留 MCP 的非文本 content 项。这样截图是工具能力，
// 不是某个固定工作流或前端按钮的特例；任何 MCP 工具均可返回图像工件。
func callMCPToolWithArtifacts(fullName, argsJSON string) (mcpToolCallResult, error) {
	conn, ok := mcpRoutes[fullName]
	if !ok {
		return mcpToolCallResult{}, fmt.Errorf("未知的 MCP 工具: %s", fullName)
	}
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return mcpToolCallResult{}, fmt.Errorf("MCP 工具参数解析失败: %w", err)
		}
	}
	if fullName == "mcp__fs__edit_file" {
		normalizeMCPEditArgs(args)
	}
	// 生图工具：模型没显式传 provider 时自动注入前端选的默认值。
	// 不走系统提示词——跟识图模型路由一个思路，模型不感知、不浪费 token。
	if fullName == "mcp__image_generate__image_generate" {
		injectImageProvider(args)
	}
	// AgentFS：真实落盘前捕获 before 快照（旁路，错误静默忽略）
	OnBeforeWrite(fullName, args)

	raw, err := conn.request("tools/call", map[string]any{
		"name": mcpRealName[fullName], "arguments": args,
	}, mcpCallTimeout(fullName))
	if err != nil {
		return mcpToolCallResult{}, err
	}

	var result struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Data     string `json:"data"`
			MimeType string `json:"mimeType"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return mcpToolCallResult{}, fmt.Errorf("MCP 结果解析失败: %w", err)
	}

	var sb strings.Builder
	images := make([]mcpImageArtifact, 0, 1)
	for _, item := range result.Content {
		if item.Type == "text" {
			sb.WriteString(item.Text)
		} else if item.Type == "image" && item.Data != "" {
			images = append(images, mcpImageArtifact{Data: item.Data, MimeType: item.MimeType})
		}
	}
	text := truncateChars(sb.String(), codeResultMaxChars)
	if result.IsError {
		return mcpToolCallResult{}, fmt.Errorf("%s", text)
	}
	// AgentFS：真实落盘成功后捕获 after 并写入影子仓审计（仅成功路径；
	// IsError / err 返回的路径文件未真正改动，不记审计）
	OnAfterWrite(fullName, args)
	return mcpToolCallResult{Text: text, Images: images}, nil
}

// normalizeMCPEditArgs 兜底纠正模型对 mcp__fs__edit_file 的参数形状混淆。
// 内置 edit_file 用扁平的 old_string/new_string；MCP filesystem 的 edit_file 实际
// schema 是 edits:[{oldText,newText}] 数组——两个工具做同一件事但形状不同，模型有
// 概率把熟悉的扁平写法套到这个工具上，导致 edits 缺失，MCP server 直接报错或空操作。
// 用户看到的现象是"第一次批准的编辑总失败，模型自己纠正后第二次才成功"——与其等
// 模型自己在下一轮改口重调（多花一次审批 + 一轮对话），这里在真正发给 MCP server
// 前把两种写法拉齐，第一次就用对的形状。
func normalizeMCPEditArgs(args map[string]any) {
	if args == nil {
		return
	}
	if _, hasEdits := args["edits"]; hasEdits {
		return
	}
	oldStr, hasOld := args["old_string"].(string)
	if !hasOld {
		oldStr, hasOld = args["oldText"].(string)
	}
	newStr, hasNew := args["new_string"].(string)
	if !hasNew {
		newStr, hasNew = args["newText"].(string)
	}
	if !hasOld && !hasNew {
		return
	}
	args["edits"] = []map[string]any{{"oldText": oldStr, "newText": newStr}}
	delete(args, "old_string")
	delete(args, "new_string")
	delete(args, "oldText")
	delete(args, "newText")
}

// injectImageProvider 在调用 image_generate 时无条件注入前端用户选的提供商。
// 工具 schema 不暴露 provider 参数——这是用户的选择，不是模型的决策。
func injectImageProvider(args map[string]any) {
	if args == nil {
		return
	}
	args["provider"] = currentImageProvider
}

// close 终止 MCP server 子进程，切换项目重建连接前调用。
func (c *mcpConn) close() {
	if c == nil || c.stdin == nil {
		return
	}
	_ = c.stdin.Close()
}
