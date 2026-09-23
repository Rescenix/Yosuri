package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

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
	// forceQuit 是显式退出放行位：托盘「退出」置位后，OnBeforeClose 不再拦截关闭。
	// 没有它的话，「关闭即缩托盘」模式下用户永远退不出应用。
	forceQuit atomic.Bool
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
	// 原生 Windows 开机自启（HKCU Run 注册表，正式版生效）
	if err := ensureAutoStart(); err != nil {
		log.Printf("⚠️ 写入开机自启失败: %v", err)
	}
	// 云端记忆同步（可选）：启动后自动拉取一次（换设备恢复记忆）
	handler.StartupMemorySyncPull()
	// 云端记忆同步定时循环（2026-08-28 用户定稿：开启即自动双向同步，不再等记忆写工具）
	handler.StartMemorySyncLoop()
	// 聚合端口调用统计：启动定时批量上报 goroutine（不阻塞，未配 key 静默跳过）
	go handler.StartAggStatsFlusher()
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

// OnBeforeClose 决定点关闭按钮时是缩到右下角托盘还是真的退出。
// Wails 的 HideWindowOnClose 是编译期写死的 true，没法运行时切换，所以改成
// 关掉它 + 挂这个回调：返回 true = 拦下关闭（此时窗口已隐藏，进程继续驻留托盘）。
// 2026-09-23 用户要求：关闭行为由用户决定（设置面板「常规 → 启动 → 关闭窗口时缩到托盘」）。
func (a *DesktopApp) OnBeforeClose(ctx context.Context) bool {
	// 托盘「退出」是明确退出，放行（否则「缩到托盘」模式下关不掉应用）
	if a.forceQuit.Load() {
		return false
	}
	// 用户选了「关闭即退出」：不拦，Wails 正常结束进程
	if !handler.CloseToTrayDesired() {
		return false
	}
	wailsruntime.WindowHide(ctx)
	return true
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
