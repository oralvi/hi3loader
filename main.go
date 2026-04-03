package main

import (
	"embed"
	"log"

	"hi3loader/internal/buildinfo"
	"hi3loader/internal/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func appTitle() string {
	title := "HI3 Loader " + buildinfo.AppVersion
	if stamp := buildinfo.EffectiveBuildStamp(); stamp != "" {
		title += " [" + stamp + "]"
	}
	return title
}

func main() {
	svc, err := service.New("config.json")
	if err != nil {
		log.Fatalf("init service: %v", err)
	}

	app := NewApp(svc)
	err = wails.Run(&options.App{
		Title:            appTitle(),
		Width:            1480,
		Height:           980,
		MinWidth:         1180,
		MinHeight:        760,
		BackgroundColour: options.NewRGB(244, 238, 228),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "hi3loader.main.instance",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				app.RevealWindow()
			},
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
