package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devin-hart/nox-maps/internal/eqlog"
	"github.com/devin-hart/nox-maps/internal/parser"
	"github.com/devin-hart/nox-maps/internal/ui"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	eqPath := "/home/wizardbeard/Games/EverQuest Project 1999"

	cwd, _ := os.Getwd()
	projectMapPath := filepath.Join(cwd, "assets", "maps")
	
	// CHANGED: Using JSON configuration
	lookupPath := filepath.Join(projectMapPath, "map_keys.json")

	fmt.Println("⚔️ Nox Maps Starting...")
	
	reader := eqlog.NewReader(eqPath)
	engine := parser.NewEngine()

	if err := reader.Start(); err != nil {
		log.Fatalf("Error starting log reader: %v", err)
	}

	go engine.ProcessLines(reader, reader.Lines)

	// Initialize UI with JSON config path
	window := ui.NewWindow(engine, projectMapPath, lookupPath)
	if err := window.Init(); err != nil {
		log.Printf("Window init warning: %v", err)
	}

	if err := ebiten.RunGame(window); err != nil {
		log.Fatal(err)
	}
}