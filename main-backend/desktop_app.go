package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"backend/internal/handler"
)

type DesktopApp struct {
	mu         sync.RWMutex
	backendURL string
	server     *http.Server
	listener   net.Listener
	ctx        context.Context
	trayOnce   sync.Once
	trayWindow uintptr
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{}
}

func (a *DesktopApp) StartBackend() error {
	// 优先用固定端口 8080（聚合 API 文档写死的地址），被占时回退随机端口
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("监听本机 API 端口失败: %w", err)
		}
	}
	server := &http.Server{
		Handler:           handler.NewAPIRouter(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	a.mu.Lock()
	a.listener = listener
	a.server = server
	a.backendURL = "http://" + listener.Addr().String()
	a.mu.Unlock()

	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("⚠️ 桌面 API 服务退出: %v", serveErr)
		}
	}()
	log.Printf("🚀 Rescene 桌面 API 已启动：%s", a.BackendURL())
	// 云端记忆同步（可选）：启动后自动拉取一次（换设备恢复记忆）
	handler.StartupMemorySyncPull()
	return nil
}

// BackendURL 由 Wails 绑定暴露给前端。前端在挂载 Vue 之前读取它，并统一改写
// fetch/EventSource/WebSocket 的 /api 请求，因此无需固定端口，也不会与开发服务冲突。
func (a *DesktopApp) BackendURL() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.backendURL
}

// Startup captures Wails' runtime context for notification-area actions.
func (a *DesktopApp) Startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
}

func (a *DesktopApp) Shutdown(ctx context.Context) {
	a.stopTray()
	_ = handler.StopPreviewBrowser()
	a.mu.RLock()
	server := a.server
	a.mu.RUnlock()
	if server == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
