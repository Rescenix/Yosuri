//go:build windows && !bindings

package main

import (
	"log"
	"os"
	"runtime"
	"sync"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	trayCallbackMessage = 0x8001 // WM_APP + 1
	trayShowCommand     = 1001
	trayQuitCommand     = 1002

	wmClose          = 0x0010
	wmDestroy        = 0x0002
	wmCommand        = 0x0111
	wmNull           = 0x0000
	wmLButtonUp      = 0x0202
	wmLButtonDblClk  = 0x0203
	wmRButtonUp      = 0x0205
	nimAdd           = 0x00000000
	nimModify        = 0x00000001
	nimDelete        = 0x00000002
	nimSetVersion    = 0x00000004
	nifMessage       = 0x00000001
	nifIcon          = 0x00000002
	nifTip           = 0x00000004
	nifState         = 0x00000008
	nisHidden        = 0x00000001
	notifyVersion    = 3
	mfString         = 0x00000000
	mfSeparator      = 0x00000800
	tpmRightButton   = 0x0002
	tpmReturnCommand = 0x0100
	idcArrow         = 32512
	idiApplication   = 32512
	csHRedraw        = 0x0002
	csVRedraw        = 0x0001
	wsOverlappedWnd  = 0x00CF0000
	cwUseDefault     = 0x80000000
	swHide           = 0
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	shell32                 = windows.NewLazySystemDLL("shell32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procGetMessage          = user32.NewProc("GetMessageW")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procPostMessage         = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procRegisterClass       = user32.NewProc("RegisterClassExW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procUnregisterClass     = user32.NewProc("UnregisterClassW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procExtractIcon         = shell32.NewProc("ExtractIconW")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")

	trayApplications sync.Map
	trayWindowProc   = windows.NewCallback(handleTrayWindowMessage)
)

type trayPoint struct {
	X int32
	Y int32
}

type trayMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   trayPoint
}

type trayWindowClass struct {
	Size       uint32
	Style      uint32
	WindowProc uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSmall  uintptr
}

type trayNotifyIconData struct {
	Size            uint32
	Window          uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        windows.GUID
	BalloonIcon     uintptr
}

func (a *DesktopApp) startTray() {
	a.trayOnce.Do(func() { go a.runTray() })
}

func (a *DesktopApp) runTray() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instance, _, instanceErr := procGetModuleHandle.Call(0)
	if instance == 0 {
		log.Printf("⚠️ 创建系统托盘失败：%v", instanceErr)
		return
	}
	className, _ := windows.UTF16PtrFromString("ResceneAgentTrayWindow")
	windowName, _ := windows.UTF16PtrFromString("Rescene Agent")
	cursor, _, _ := procLoadCursor.Call(0, idcArrow)
	icon := extractApplicationIcon(instance)

	class := trayWindowClass{
		Size:       uint32(unsafe.Sizeof(trayWindowClass{})),
		Style:      csHRedraw | csVRedraw,
		WindowProc: trayWindowProc,
		Instance:   instance,
		Icon:       icon,
		Cursor:     cursor,
		Background: 6,
		ClassName:  className,
		IconSmall:  icon,
	}
	if atom, _, registerErr := procRegisterClass.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		log.Printf("⚠️ 创建系统托盘失败：注册窗口类：%v", registerErr)
		return
	}
	defer procUnregisterClass.Call(uintptr(unsafe.Pointer(className)), instance)

	hwnd, _, createErr := procCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)),
		wsOverlappedWnd, cwUseDefault, cwUseDefault, cwUseDefault, cwUseDefault,
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		log.Printf("⚠️ 创建系统托盘失败：创建消息窗口：%v", createErr)
		return
	}
	procShowWindow.Call(hwnd, swHide)
	procUpdateWindow.Call(hwnd)
	defer func() {
		trayApplications.Delete(hwnd)
		a.mu.Lock()
		a.trayWindow = 0
		a.mu.Unlock()
	}()

	iconData := trayNotifyIconData{
		Size:            uint32(unsafe.Sizeof(trayNotifyIconData{}) - unsafe.Sizeof(uintptr(0))),
		Window:          hwnd,
		ID:              100,
		Flags:           nifMessage | nifState,
		CallbackMessage: trayCallbackMessage,
		State:           nisHidden,
		StateMask:       nisHidden,
	}
	if ok, _, notifyErr := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&iconData))); ok == 0 {
		log.Printf("⚠️ 创建系统托盘失败：添加通知区图标：%v", notifyErr)
		procDestroyWindow.Call(hwnd)
		return
	}
	defer procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&iconData)))
	iconData.Version = notifyVersion
	procShellNotifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(&iconData)))
	iconData.Flags = nifIcon | nifTip | nifState
	iconData.Icon = icon
	iconData.State = 0
	iconData.StateMask = nisHidden
	copy(iconData.Tip[:], windows.StringToUTF16("Rescene Agent"))
	if ok, _, notifyErr := procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&iconData))); ok == 0 {
		log.Printf("⚠️ 创建系统托盘失败：显示通知区图标：%v", notifyErr)
		procDestroyWindow.Call(hwnd)
		return
	}
	trayApplications.Store(hwnd, a)
	a.mu.Lock()
	a.trayWindow = hwnd
	a.mu.Unlock()

	var message trayMessage
	for {
		result, _, messageErr := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			if int32(result) < 0 {
				log.Printf("⚠️ 系统托盘消息循环退出：%v", messageErr)
			}
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func extractApplicationIcon(instance uintptr) uintptr {
	if executable, err := os.Executable(); err == nil {
		if path, pathErr := windows.UTF16PtrFromString(executable); pathErr == nil {
			if icon, _, _ := procExtractIcon.Call(instance, uintptr(unsafe.Pointer(path)), 0); icon > 1 {
				return icon
			}
		}
	}
	icon, _, _ := procLoadIcon.Call(0, idiApplication)
	return icon
}

func handleTrayWindowMessage(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	appValue, _ := trayApplications.Load(hwnd)
	app, _ := appValue.(*DesktopApp)
	switch message {
	case trayCallbackMessage:
		if app == nil {
			break
		}
		switch uint32(lParam) {
		case wmLButtonUp, wmLButtonDblClk:
			go app.showWindow()
		case wmRButtonUp:
			showTrayMenu(hwnd, app)
		}
		return 0
	case wmCommand:
		if app != nil {
			handleTrayCommand(uint32(wParam)&0xffff, app)
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func showTrayMenu(hwnd uintptr, app *DesktopApp) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	showLabel, _ := windows.UTF16PtrFromString("显示主窗口")
	quitLabel, _ := windows.UTF16PtrFromString("退出")
	procAppendMenu.Call(menu, mfString, trayShowCommand, uintptr(unsafe.Pointer(showLabel)))
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	procAppendMenu.Call(menu, mfString, trayQuitCommand, uintptr(unsafe.Pointer(quitLabel)))

	var point trayPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	procSetForegroundWindow.Call(hwnd)
	command, _, _ := procTrackPopupMenu.Call(
		menu, tpmRightButton|tpmReturnCommand,
		uintptr(point.X), uintptr(point.Y), 0, hwnd, 0,
	)
	procPostMessage.Call(hwnd, wmNull, 0, 0)
	handleTrayCommand(uint32(command), app)
}

func handleTrayCommand(command uint32, app *DesktopApp) {
	switch command {
	case trayShowCommand:
		go app.showWindow()
	case trayQuitCommand:
		go app.requestQuit()
	}
}

func (a *DesktopApp) showWindow() {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()
	if ctx == nil {
		return
	}
	wailsruntime.WindowShow(ctx)
	wailsruntime.WindowUnminimise(ctx)
}

func (a *DesktopApp) requestQuit() {
	a.mu.RLock()
	ctx := a.ctx
	a.mu.RUnlock()
	if ctx != nil {
		wailsruntime.Quit(ctx)
	}
}

func (a *DesktopApp) stopTray() {
	a.mu.RLock()
	hwnd := a.trayWindow
	a.mu.RUnlock()
	if hwnd != 0 {
		procPostMessage.Call(hwnd, wmClose, 0, 0)
	}
}
