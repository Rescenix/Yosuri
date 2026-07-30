package handler

// 真实交互式终端 —— GET /api/terminal/stream?id=xxx（SSE 输出）+ POST /api/terminal/input（写 stdin）。
//
// 没有接 ConPTY，就是给 powershell.exe 接了两根管子（stdin 写、stdout/stderr 读）。
// 实测过：交互式 powershell（不带 -Command，从 stdin 一行行读）即使 stdin/stdout 都是
// 管道而不是真实控制台，依然会把 "PS C:\...\> 命令" 提示符和命令回显、输出全部照常写到
// stdout——所以前端不需要自己伪造回显，直接把 stdout 原样喂给终端窗口就是真实效果。
// 代价：没有 ANSI 全彩、没有 less/vim 这类需要真终端的全屏程序，但日常跑命令、看构建
// 输出、cd 状态跨命令保持，这些核心诉求都是真的，不是之前那个纯装饰的静态占位符。
//
// 会话按 id 常驻：面板关闭只断 SSE 连接，不杀进程，工作目录/环境变量状态原地保留，
// 重新打开面板能接着看见之前的滚屏历史（scrollback 环形缓冲）。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

const (
	termScrollbackCap = 64 * 1024 // 滚屏缓冲上限（字节），超出丢最旧的
	termSubBufSize    = 256       // 每个订阅者的 channel 缓冲，防止一次输出量大时互相阻塞
)

type termSession struct {
	id    string
	cmd   *exec.Cmd
	stdin io.WriteCloser // 直接写这根管子，不加 bufio——缓冲会让命令"发出去了但没送到"，破坏交互感

	mu         sync.Mutex
	scrollback []byte
	subs       map[chan []byte]bool
	closed     bool
}

// jsonEscapeTermChunk 把任意字节安全塞进一行 SSE data 字段——终端输出可能带控制字符、
// 换行、非法 UTF-8（Windows 控制台编码不保证是 UTF-8），json.Marshal 的字符串转义
// 已经处理了换行/引号/控制字符，比自己手搓转义规则可靠
func jsonEscapeTermChunk(p []byte) []byte {
	b, _ := json.Marshal(string(p))
	return b
}

func (s *termSession) broadcast(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	cp := make([]byte, len(p))
	copy(cp, p)
	s.scrollback = append(s.scrollback, cp...)
	if len(s.scrollback) > termScrollbackCap {
		s.scrollback = s.scrollback[len(s.scrollback)-termScrollbackCap:]
	}
	for ch := range s.subs {
		select {
		case ch <- cp:
		default:
			// 订阅者跟不上，丢这一片，不阻塞终端进程本身的输出
		}
	}
}

// Write 让 termSession 自己就是 cmd.Stdout/cmd.Stderr——不用额外开 goroutine 转发
func (s *termSession) Write(p []byte) (int, error) {
	s.broadcast(p)
	return len(p), nil
}

func (s *termSession) subscribe() chan []byte {
	ch := make(chan []byte, termSubBufSize)
	s.mu.Lock()
	s.subs[ch] = true
	s.mu.Unlock()
	return ch
}

func (s *termSession) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	delete(s.subs, ch)
	s.mu.Unlock()
}

func (s *termSession) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(s.scrollback))
	copy(cp, s.scrollback)
	return cp
}

var (
	termRegistryMu sync.Mutex
	termRegistry   = map[string]*termSession{}
)

func getOrCreateTermSession(id string) (*termSession, error) {
	termRegistryMu.Lock()
	defer termRegistryMu.Unlock()

	if s, ok := termRegistry[id]; ok && !s.closed {
		return s, nil
	}

	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile")
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	cmd.Dir = core.GetProjectRoot()

	s := &termSession{id: id, cmd: cmd, subs: map[chan []byte]bool{}}
	cmd.Stdout = s
	cmd.Stderr = s

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 stdin 管道失败: %w", err)
	}
	s.stdin = stdinPipe

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动终端进程失败: %w", err)
	}

	termRegistry[id] = s

	go func() {
		cmd.Wait()
		s.mu.Lock()
		s.closed = true
		for ch := range s.subs {
			close(ch)
		}
		s.subs = map[chan []byte]bool{}
		s.mu.Unlock()
	}()

	return s, nil
}

// HandleTerminalStream GET /api/terminal/stream?id=xxx —— SSE 输出流
func HandleTerminalStream(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 参数必填"})
		return
	}

	s, err := getOrCreateTermSession(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 先把滚屏历史整段回放一次，重新打开面板能看见之前的输出
	if backlog := s.snapshot(); len(backlog) > 0 {
		fmt.Fprintf(c.Writer, "event: chunk\ndata: %s\n\n", jsonEscapeTermChunk(backlog))
		c.Writer.Flush()
	}

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case p, ok := <-ch:
			if !ok {
				fmt.Fprintf(c.Writer, "event: exit\ndata: {}\n\n")
				c.Writer.Flush()
				return
			}
			fmt.Fprintf(c.Writer, "event: chunk\ndata: %s\n\n", jsonEscapeTermChunk(p))
			c.Writer.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": ping\n\n")
			c.Writer.Flush()
		}
	}
}

// HandleTerminalInput POST /api/terminal/input {"id": "...", "data": "ls\n"}
func HandleTerminalInput(c *gin.Context) {
	var body struct {
		ID   string `json:"id" binding:"required"`
		Data string `json:"data"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id/data 参数错误"})
		return
	}

	s, err := getOrCreateTermSession(body.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		c.JSON(http.StatusGone, gin.H{"error": "终端会话已结束"})
		return
	}
	if _, err := s.stdin.Write([]byte(body.Data)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入终端失败: " + err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

// HandleTerminalClose POST /api/terminal/close {"id": "..."} —— 显式结束会话，杀掉进程
func HandleTerminalClose(c *gin.Context) {
	var body struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 参数错误"})
		return
	}

	termRegistryMu.Lock()
	s, ok := termRegistry[body.ID]
	if ok {
		delete(termRegistry, body.ID)
	}
	termRegistryMu.Unlock()

	if ok && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	c.Status(http.StatusOK)
}
