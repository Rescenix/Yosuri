//go:build windows && !bindings

package main

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsTrayLifecycle(t *testing.T) {
	shellTrayClass, _ := windows.UTF16PtrFromString("Shell_TrayWnd")
	findWindow := windows.NewLazySystemDLL("user32.dll").NewProc("FindWindowW")
	if shellTray, _, _ := findWindow.Call(uintptr(unsafe.Pointer(shellTrayClass)), 0); shellTray == 0 {
		t.Skip("Windows Explorer notification area is unavailable in this test session")
	}

	app := NewDesktopApp()
	app.startTray()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.mu.RLock()
		hwnd := app.trayWindow
		app.mu.RUnlock()
		if hwnd != 0 {
			app.stopTray()
			waitForTrayToStop(t, app)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Windows notification-area icon was not registered")
}

func waitForTrayToStop(t *testing.T, app *DesktopApp) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.mu.RLock()
		hwnd := app.trayWindow
		app.mu.RUnlock()
		if hwnd == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Windows notification-area icon did not shut down")
}
