package main

import (
	"embed"
	"log"
	"os"
	"time"

	"backend/internal/handler"
	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

func main() {
	_ = godotenv.Load()
	// 待应用热补丁检查：后台下载的补丁（正式版/测试版）在启动时一律静默自动应用
	// （2026-08-28 用户定稿：不再发测试版，正式版同样免手动点击直接热更新）。
	// 必须在 wails.Run 之前：检测到待应用补丁时整个 GUI 不启动，静默完成替换。
	// -no-hotpatch：热补丁脚本 :failed 拉起旧版时跳过自动应用，避免循环（见 desktop_launch.go）。
	if !hasNoHotPatchFlag(os.Args[1:]) && handler.ApplyPendingHotPatch() {
		return
	}
	// 延迟确保快捷方式存在（避免启动时弹 PowerShell 黑窗，2026-09-01 实锤）
	go func() {
		time.Sleep(5 * time.Second)
		ensureDesktopShortcuts()
	}()
	app := NewDesktopApp()
	if err := app.StartBackend(); err != nil {
		log.Fatal(err)
	}
	app.startTray()
	app.startOverlay()
	err := wails.Run(&options.App{
		Title:             "Yosuri",
		Width:             1200,
		Height:            800,
		MinWidth:          1024,
		MinHeight:         720,
		WindowStartState:  options.Normal,
		StartHidden:       hasBackgroundFlag(os.Args[1:]),
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 248, G: 247, B: 252, A: 255},
		AssetServer:       &assetserver.Options{Assets: frontendAssets},
		OnStartup:         app.Startup,
		OnShutdown:        app.Shutdown,
		Bind:              []interface{}{app},
		Windows:           &windows.Options{Theme: windows.SystemDefault},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "com.rescenix.rescene-agent",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.showWindow() },
		},
		EnableDefaultContextMenu: false,
	})
	if err != nil {
		app.Shutdown(nil)
		log.Fatal(err)
	}
}
