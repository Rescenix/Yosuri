package main

import (
	"embed"
	"log"
	"os"

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
	// 待应用热补丁检查：上次会话选择了「下次启动时更新」→ 启动即应用（替换 exe 后自动重启）。
	// 必须在 wails.Run 之前：检测到待应用补丁时整个 GUI 不启动，静默完成替换。
	if handler.ApplyPendingHotPatch() {
		return
	}
	app := NewDesktopApp()
	if err := app.StartBackend(); err != nil {
		log.Fatal(err)
	}
	app.startTray()
	err := wails.Run(&options.App{
		Title:             "Rescene Agent",
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
