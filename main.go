package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  windowTitle,
		Width:  600, // just fits the controls; see frontend/playwright.config.cjs
		Height: 632,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:   &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:          app.startup,
		OnDomReady:         app.ready,
		OnShutdown:         app.shutdown,
		OnBeforeClose:      app.beforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "4a162763-cda7-4354-aea4-62cf92b5591d"},
		Bind: []interface{}{
			app,
		},
		DisableResize: true,
		Frameless:     true, // only false for debugging
		AlwaysOnTop:   true,
		Windows: &windows.Options{
			DisableFramelessWindowDecorations: true,
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			Theme:                             windows.Dark,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
