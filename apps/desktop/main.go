package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "Advance HRIS",
		Width:            1440,
		Height:           920,
		MinWidth:         1120,
		MinHeight:        720,
		DisableResize:    false,
		Frameless:        false,
		BackgroundColour: &options.RGBA{R: 8, G: 13, B: 24, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
	})

	if err != nil {
		log.Fatal(err)
	}
}
