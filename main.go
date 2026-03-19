package main

import (
	"embed"
	"log"
	"os"

	"hi3loader/internal/bridge"
	"hi3loader/internal/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	handled, err := bridge.HandleAuxRuntime(os.Args[1:], os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	if handled {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("resolve executable path: %v", err)
	}

	svc, err := service.NewWithOptions("config.json", service.Options{
		BridgeExecutable: exePath,
		RequireBridge:    true,
	})
	if err != nil {
		log.Fatalf("init service: %v", err)
	}

	app := NewApp(svc)
	err = wails.Run(&options.App{
		Title:            "HI3 Loader 1.1.0",
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
