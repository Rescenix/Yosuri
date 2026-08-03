package main

import (
	"embed"
	"log"
	"os"

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
