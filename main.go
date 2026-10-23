package main

import (
	"embed"
	"log"

	"github.com/lurp/hermit/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	service := app.New()
	app := application.New(application.Options{
		Name:        "Hermit",
		Description: "Clash Verge Rev 固定代理端口管理器",
		Services:    []application.Service{application.NewService(service)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:    "Hermit",
		Width:    1040,
		Height:   700,
		MinWidth: 860,
		MinHeight: 560,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
