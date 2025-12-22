package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devin-hart/nox-maps/internal/config"
	"github.com/devin-hart/nox-maps/internal/eqlog"
	"github.com/devin-hart/nox-maps/internal/parser"
	"github.com/devin-hart/nox-maps/internal/ui"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	cfg := config.Load()

	cwd, _ := os.Getwd()
	projectMapPath := filepath.Join(cwd, "assets", "maps")

	// CHANGED: Using JSON configuration
	lookupPath := filepath.Join(projectMapPath, "map_keys.json")

	fmt.Println("⚔️ Nox Maps Starting...")

	var reader *eqlog.Reader
	engine := parser.NewEngine()

	// Only initialize log reader if path is configured
	if cfg.EQPath != "" {
		reader = eqlog.NewReader(cfg.EQPath)
		if err := reader.Start(); err != nil {
			log.Printf("Warning: Error starting log reader: %v", err)
		} else {
			go engine.ProcessLines(reader, reader.Lines)
		}
	} else {
		fmt.Println("⚠️  No EQ path configured. Please set it in the menu bar.")
	}

	// Initialize UI with JSON config path
	window := ui.NewWindow(engine, projectMapPath, lookupPath, cfg)
	if err := window.Init(); err != nil {
		log.Printf("Window init warning: %v", err)
	}

	if err := ebiten.RunGame(window); err != nil {
		log.Fatal(err)
	}
}