//go:build windows && !bindings

// 悬浮球演示窗口：独立于 Wails 主窗口的第二个原生窗口(WS_POPUP + 置顶 + 不进任务栏),
// 内嵌一个单独的 WebView2 实例加载本机 /overlay 页面。主窗口关闭到托盘时自动出现,
// 重新打开主窗口时自动收起——不改动主窗口/主聊天界面任何一行代码(方案 B，见对话记录)。
//
// 关键坑(实现前查证过):
//   1. Wails v2 只有一个窗口，没有官方多窗口 API——这个悬浮窗完全是手搓的第二个
//      Win32 窗口，复用 desktop_tray_windows.go 里已经验证过的"独立线程 + 自己的
//      消息循环"模式（WebView2 是 STA COM 对象，必须固定在创建它的那个线程上跑）。
//   2. WebView2 的 Chromium.DataPath 不显式指定的话默认落在同一个
//      %AppData%/<exe名> 目录——和 Wails 主窗口自己的 WebView2 实例共用一个 profile
//      目录，两个独立 environment 抢占同一份 profile 容易创建失败，这里显式指到
//      一个专用子目录，彻底避免冲突。
//   3. windows.NewCallback 生成的回调 thunk 是不可回收的全局资源（有上限，约
//      2000 个）；EnumWindows 的回调必须是包级变量只创建一次，不能在轮询循环里
//      每次现造一个闭包传进去，否则跑几分钟就把回调池耗尽。
//   4. ICoreWebView2Controller.IsVisible 是独立于原生窗口 WS_VISIBLE 的另一个
//      开关，默认创建出来是不可见的——必须显式 chromium.Show()，否则窗口存在、
//      置顶、命中测试都正常，但画面整个不渲染(肉眼和各种截屏方式都验证过)。
//   5. 拖动/缩放走 Windows 原生的 WM_NCLBUTTONDOWN + 命中测试码(HTCAPTION/
//      HTBOTTOMRIGHT 等)技巧：ReleaseCapture 后 SendMessage 一条 WM_NCLBUTTONDOWN，
//      DefWindowProc 会接管一个阻塞到松开鼠标为止的模态拖动/缩放循环，不需要自己
//      写 mousemove 累加逻辑。前端只需要在“按下但还没明显移动”和“按下就是拖动”
//      之间做一个像素阈值判定，避免把正常点击(展开/收起)也误当成拖动消费掉。
//   6. 真透明背景(用户要求: 展开面板要能看见后面的游戏画面，不能是一块不透明的
//      深色卡片)不是靠 SetWindowRgn 硬裁剪能做到的——那只是"整块区域外直接砍掉"，
//      区域内部还是完全不透明。真正的每像素 alpha 透明要同时满足两个条件：
//        a) 窗口带 WS_EX_NOREDIRECTIONBITMAP，让 DWM 不给它分配普通重定向位图，
//           WebView2 转而走 DirectComposition 合成自己的内容；
//        b) chromium.SetBackgroundColour(0,0,0,0) 告诉 WebView2 页面背景本身是透明的。
//      两者缺一都会退化成"整块窗口纯色/纯黑"。做到这两点后，窗口本身不再需要任何
//      形状裁剪，球的圆形/面板的方形完全交给页面 CSS(border-radius + 透明背景)处理，
//      而且是真正抗锯齿的，比 SetWindowRgn 的硬边缘区域好看得多。
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"backend/internal/handler"
	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

const (
	overlayClassName = "ResceneAgentOverlayWindow"
	overlayBallSize  = 64
	overlayPanelW    = 340
	overlayPanelH    = 440
	overlayPanelMinW = 280
	overlayPanelMinH = 320
	overlayPanelMaxW = 560
	overlayPanelMaxH = 760
	overlayMargin    = 24

	wmGetMinMaxInfo = 0x0024
	wmNCLButtonDown = 0x00A1
	htCaption       = 2
	htLeft          = 10
	htRight         = 11
	htTop           = 12
	htTopLeft       = 13
	htTopRight      = 14
	htBottom        = 15
	htBottomLeft    = 16
	htBottomRight   = 17
)

var (
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetClassName             = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procReleaseCapture           = user32.NewProc("ReleaseCapture")
	procSendMessageW             = user32.NewProc("SendMessageW")

	overlayWindowProc = windows.NewCallback(handleOverlayWindowMessage)
	enumWindowsProc    = windows.NewCallback(enumWindowsCallback)

	overlayOnce sync.Once

	overlayMu       sync.Mutex
	overlayHwnd     uintptr
	overlayChromium *edge.Chromium
	overlayExpanded bool

	overlayReadyCh = make(chan struct{})

	enumMu           sync.Mutex
	enumTargetPid    uint32
	enumFoundVisible bool

	overlayLayoutMu      sync.Mutex
	currentOverlayLayout overlayLayout

	edgeHitTest = map[string]uintptr{
		"n": htTop, "s": htBottom, "e": htRight, "w": htLeft,
		"ne": htTopRight, "nw": htTopLeft, "se": htBottomRight, "sw": htBottomLeft,
	}
)

// startOverlay 只在悬浮球开关打开时才创建悬浮窗/WebView2 实例——这是测试功能，
// 默认关闭，不能让没打开过设置的普通用户也悄悄多出一个常驻 WebView2 进程。
// 开关目前只在启动时读一次：设置面板改了要重启应用才生效（见 overlay_config.go）。
func (a *DesktopApp) startOverlay() {
	if !handler.OverlayEnabled() {
		return
	}
	overlayOnce.Do(func() {
		go runOverlay(a)
		go a.watchMainWindowVisibility()
	})
}

// ---------- 布局持久化(位置 + 展开尺寸，跨重启记住用户拖拽/缩放的结果) ----------

type overlayLayout struct {
	X      int32 `json:"x"`
	Y      int32 `json:"y"`
	PanelW int32 `json:"panel_w"`
	PanelH int32 `json:"panel_h"`
}

func overlayLayoutPath() string {
	return filepath.Join(os.Getenv("AppData"), "rescene-overlay-webview2", "overlay_layout.json")
}

func defaultOverlayLayout() overlayLayout {
	sw, sh := screenSize()
	return overlayLayout{
		X:      sw - overlayBallSize - overlayMargin,
		Y:      sh - overlayBallSize - overlayMargin - 40,
		PanelW: overlayPanelW,
		PanelH: overlayPanelH,
	}
}

func clampInt32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func loadOverlayLayout() overlayLayout {
	def := defaultOverlayLayout()
	data, err := os.ReadFile(overlayLayoutPath())
	if err != nil {
		return def
	}
	var l overlayLayout
	if err := json.Unmarshal(data, &l); err != nil {
		return def
	}
	l.PanelW = clampInt32(l.PanelW, overlayPanelMinW, overlayPanelMaxW)
	l.PanelH = clampInt32(l.PanelH, overlayPanelMinH, overlayPanelMaxH)
	sw, sh := screenSize()
	if l.X < 0 || l.Y < 0 || l.X > sw-20 || l.Y > sh-20 {
		// 上次保存的位置超出了当前屏幕范围(比如换了分辨率/拔了副屏)，回退默认角落
		l.X, l.Y = def.X, def.Y
	}
	return l
}

func saveOverlayLayout(x, y, w, h int32) {
	overlayLayoutMu.Lock()
	l := currentOverlayLayout
	l.X, l.Y = x, y
	overlayMu.Lock()
	expanded := overlayExpanded
	overlayMu.Unlock()
	if expanded {
		// 拖球体时 w/h 是球的 64x64，不能当成面板尺寸存下来；只有展开态才更新
		l.PanelW = clampInt32(w, overlayPanelMinW, overlayPanelMaxW)
		l.PanelH = clampInt32(h, overlayPanelMinH, overlayPanelMaxH)
	}
	currentOverlayLayout = l
	overlayLayoutMu.Unlock()

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(overlayLayoutPath()), 0o755)
	_ = os.WriteFile(overlayLayoutPath(), data, 0o644)
}

func screenSize() (int32, int32) {
	const smCxScreen, smCyScreen = 0, 1
	w, _, _ := procGetSystemMetrics.Call(smCxScreen)
	h, _, _ := procGetSystemMetrics.Call(smCyScreen)
	return int32(w), int32(h)
}

func overlayBallRect() (x, y, w, h int32) {
	overlayLayoutMu.Lock()
	l := currentOverlayLayout
	overlayLayoutMu.Unlock()
	return l.X, l.Y, overlayBallSize, overlayBallSize
}

func overlayPanelRect() (x, y, w, h int32) {
	overlayLayoutMu.Lock()
	l := currentOverlayLayout
	overlayLayoutMu.Unlock()
	return l.X, l.Y, l.PanelW, l.PanelH
}

type overlayMinMaxInfo struct {
	ptReserved     [2]int32
	ptMaxSize      [2]int32
	ptMaxPosition  [2]int32
	ptMinTrackSize [2]int32
	ptMaxTrackSize [2]int32
}

func handleOverlayWindowMessage(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmDestroy:
		return 0
	case wmGetMinMaxInfo:
		// 只约束展开态的可拖拽缩放范围；收起态的球从不走用户交互式缩放，不受影响。
		info := (*overlayMinMaxInfo)(unsafe.Pointer(lParam))
		info.ptMinTrackSize = [2]int32{overlayPanelMinW, overlayPanelMinH}
		info.ptMaxTrackSize = [2]int32{overlayPanelMaxW, overlayPanelMaxH}
		return 0
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func createOverlayWindow(instance uintptr) uintptr {
	className, _ := windows.UTF16PtrFromString(overlayClassName)
	windowName, _ := windows.UTF16PtrFromString("Rescene Agent Overlay")
	cursor, _, _ := procLoadCursor.Call(0, idcArrow)
	icon := extractApplicationIcon(instance)

	class := trayWindowClass{
		Size:       uint32(unsafe.Sizeof(trayWindowClass{})),
		Style:      csHRedraw | csVRedraw,
		WindowProc: overlayWindowProc,
		Instance:   instance,
		Icon:       icon,
		Cursor:     cursor,
		Background: 0,
		ClassName:  className,
		IconSmall:  icon,
	}
	if atom, _, registerErr := procRegisterClass.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		log.Printf("⚠️ 创建悬浮窗失败：注册窗口类：%v", registerErr)
		return 0
	}

	const (
		wsPopup               = 0x80000000
		wsExTopMost           = 0x00000008
		wsExToolWindow        = 0x00000080
		wsExNoRedirectionBmp  = 0x00200000 // 真透明必需：不让 DWM 给这个窗口分配普通 GDI 重定向表面，
		// 交给 WebView2 自己走 DirectComposition 合成，这样它按透明背景渲染出来的像素才能
		// 真正跟桌面/游戏画面混合，而不是被一块不透明的重定向位图挡住(实测过, 缺了这个
		// flag 单靠 SetBackgroundColour(alpha=0) 没用，窗口区域还是纯色/纯黑)。
	)
	x, y, w, h := overlayBallRect()
	hwnd, _, createErr := procCreateWindowEx.Call(
		uintptr(wsExTopMost|wsExToolWindow|wsExNoRedirectionBmp),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(wsPopup),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		log.Printf("⚠️ 创建悬浮窗失败：创建窗口：%v", createErr)
	}
	return hwnd
}

func runOverlay(a *DesktopApp) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// WebView2 是 STA COM 对象；这个 goroutine 被 LockOSThread 固定到一个全新的
	// OS 线程上，该线程默认没有初始化 COM 套间，必须显式 CoInitializeEx，否则
	// 环境创建会报 "CoInitialize has not been called"（实测踩到过）。
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil {
		log.Printf("ℹ️ 悬浮窗线程 CoInitializeEx: %v（S_FALSE=已初始化，可忽略）", err)
	}

	overlayLayoutMu.Lock()
	currentOverlayLayout = loadOverlayLayout()
	overlayLayoutMu.Unlock()

	instance, _, instanceErr := procGetModuleHandle.Call(0)
	if instance == 0 {
		log.Printf("⚠️ 创建悬浮窗失败：%v", instanceErr)
		return
	}
	hwnd := createOverlayWindow(instance)
	if hwnd == 0 {
		return
	}

	chromium := edge.NewChromium()
	chromium.DataPath = filepath.Join(os.Getenv("AppData"), "rescene-overlay-webview2")
	// 64x64 的球/一个小面板不需要 GPU 合成；关掉后走软件光栅化，
	// 真透明要走 DirectComposition 合成路径，跟 GPU 加速是绑在一起的——这里不能再关 GPU
	// (之前调试截屏问题时关过，那次的根因其实是漏调 chromium.Show()，跟 GPU 无关，已改正)。
	chromium.MessageCallback = func(msg string, _ *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		var m struct {
			Action string `json:"action"`
			Edge   string `json:"edge"`
		}
		if err := json.Unmarshal([]byte(msg), &m); err != nil {
			return
		}
		switch m.Action {
		case "toggle":
			toggleOverlay(hwnd, chromium)
		case "drag":
			dragOrResizeOverlay(hwnd, chromium, htCaption)
		case "resize":
			if ht, ok := edgeHitTest[m.Edge]; ok {
				dragOrResizeOverlay(hwnd, chromium, ht)
			}
		case "disable":
			// 右键球/面板的快捷关闭：立刻落盘关掉开关 + 收起窗口，不用去设置面板绕一圈。
			// watchMainWindowVisibility 的轮询也会读到这个开关变化，这里主动 hideOverlay
			// 只是让收起动作不用等下一个 500ms 轮询周期，体验更即时。
			if err := handler.DisableOverlay(); err != nil {
				log.Printf("⚠️ 悬浮球右键关闭失败：%v", err)
				return
			}
			hideOverlay()
		}
	}
	chromium.SetErrorCallback(func(err error) {
		log.Printf("⚠️ 悬浮窗 WebView2 错误：%v", err)
	})
	if !chromium.Embed(hwnd) {
		log.Printf("⚠️ 悬浮窗 WebView2 初始化失败")
		return
	}
	chromium.Resize()
	chromium.SetBackgroundColour(0, 0, 0, 0) // alpha=0：配合 WS_EX_NOREDIRECTIONBITMAP 才是真透明
	// 见文件头坑 4：控制器可见性要单独打开。
	if err := chromium.Show(); err != nil {
		log.Printf("⚠️ 悬浮窗 controller.PutIsVisible(true) 失败：%v", err)
	}

	backendURL := a.BackendURL()
	if backendURL == "" {
		backendURL = "http://127.0.0.1:8080"
	}
	chromium.Navigate(backendURL + "/overlay")

	overlayMu.Lock()
	overlayHwnd = hwnd
	overlayChromium = chromium
	overlayMu.Unlock()
	close(overlayReadyCh)

	var message trayMessage
	for {
		result, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func toggleOverlay(hwnd uintptr, chromium *edge.Chromium) {
	overlayMu.Lock()
	overlayExpanded = !overlayExpanded
	nowExpanded := overlayExpanded
	overlayMu.Unlock()

	const swpNoZOrder = 0x0004
	var x, y, w, h int32
	if nowExpanded {
		x, y, w, h = overlayPanelRect()
	} else {
		x, y, w, h = overlayBallRect()
	}
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoZOrder)
	chromium.Resize()
}

// dragOrResizeOverlay 用 Windows 原生"非客户区拖动"技巧接管一次移动/缩放手势：
// ReleaseCapture 后送一条 WM_NCLBUTTONDOWN(命中测试码 = ht)，DefWindowProc 会
// 自己跑一个阻塞到松开鼠标为止的模态循环，行为和拖窗口标题栏/拖边框完全一致，
// 不用自己在 mousemove 里累加坐标。SendMessageW 会一直阻塞到手势结束才返回，
// 返回后窗口新的位置/尺寸已经落定，正好在这里读出来落盘。
func dragOrResizeOverlay(hwnd uintptr, chromium *edge.Chromium, ht uintptr) {
	var pt trayPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procReleaseCapture.Call()
	lparam := uintptr(uint32(pt.X)) | uintptr(uint32(pt.Y))<<16
	procSendMessageW.Call(hwnd, wmNCLButtonDown, ht, lparam)

	var r struct{ Left, Top, Right, Bottom int32 }
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	x, y, w, h := r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top
	saveOverlayLayout(x, y, w, h)

	if ht != htCaption {
		// 缩放会改变客户区尺寸，WebView2 内容要重新贴合(不再需要重算裁剪区域——
		// 形状现在完全交给 CSS + 真透明背景处理，见文件头坑 6)。
		chromium.Resize()
	}
}

const ovSwShow = 5

func showOverlay() {
	overlayMu.Lock()
	hwnd := overlayHwnd
	overlayMu.Unlock()
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, uintptr(ovSwShow))
}

func hideOverlay() {
	overlayMu.Lock()
	hwnd := overlayHwnd
	overlayMu.Unlock()
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, uintptr(swHide))
}

// enumWindowsCallback 是包级单例回调（见文件头坑 3），轮询时只改 enumTargetPid/
// enumFoundVisible 这两个包级变量，不重新创建回调。
func enumWindowsCallback(hwnd uintptr, _ uintptr) uintptr {
	var winPid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&winPid)))
	if winPid != enumTargetPid {
		return 1
	}
	classBuf := make([]uint16, 256)
	procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&classBuf[0])), uintptr(len(classBuf)))
	class := windows.UTF16ToString(classBuf)
	if class == overlayClassName || class == "ResceneAgentTrayWindow" {
		return 1 // 跳过悬浮窗和托盘消息窗口自己，只关心真正的主窗口
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible != 0 {
		enumFoundVisible = true
		return 0
	}
	return 1
}

func isMainWindowVisible(pid uint32) bool {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumTargetPid = pid
	enumFoundVisible = false
	procEnumWindows.Call(enumWindowsProc, 0)
	return enumFoundVisible
}

// watchMainWindowVisibility 轮询主窗口可见性，主窗口一隐藏（关闭到托盘）就现出
// 悬浮球，重新显示主窗口就收起——纯轮询，不 hook 主窗口任何消息，零风险。
func (a *DesktopApp) watchMainWindowVisibility() {
	<-overlayReadyCh
	time.Sleep(3 * time.Second) // 让主窗口先正常完成启动展示，避免误判成"已隐藏"闪一下悬浮球
	pid := windows.GetCurrentProcessId()
	lastVisible := true
	overlayShown := false
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		// 实时查开关，不缓存：设置面板关掉、或悬浮球右键"关闭"之后要立刻收起，
		// 不能等下次启动才生效（用户实测反馈过这个问题）。开启则仍要重启才建窗口
		// （startOverlay 只在启动时判断一次），这里只负责"关掉能马上生效"这半边。
		if !handler.OverlayEnabled() {
			if overlayShown {
				hideOverlay()
				overlayShown = false
			}
			continue
		}
		visible := isMainWindowVisible(pid)
		if visible == lastVisible {
			continue
		}
		lastVisible = visible
		if visible {
			hideOverlay()
			overlayShown = false
		} else {
			showOverlay()
			overlayShown = true
		}
	}
}
