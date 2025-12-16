package ui

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"strings"

	"github.com/devin-hart/nox-maps/internal/maps"
	"github.com/devin-hart/nox-maps/internal/parser"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Breadcrumb struct {
	X, Y float64
}

type Window struct {
	Parser     *parser.Engine
	CurrentMap *maps.ZoneMap
	ZoneLookup map[string]string
	MapDir     string
	
	Zoom        float64
	IsDragging  bool
	DragStartX  int
	DragStartY  int
	OffsetX     float64
	OffsetY     float64
	
	// VISUAL SETTINGS
	BackgroundAlpha uint8 
	ShowTrail       bool 
	ShowLabels      bool 

	// PLAYER CALIBRATION
	PlayerMultX float64 
	PlayerMultY float64 
	PlayerSwap  bool    
	
	Trail []Breadcrumb
	Width, Height int
}

func NewWindow(p *parser.Engine, mapDir, lookupPath string) *Window {
	lookup, err := maps.LoadZoneLookup(lookupPath)
	if err != nil {
		fmt.Printf("⚠️ Warning: Could not load map_keys.ini: %v\n", err)
		lookup = make(map[string]string)
	} else {
		fmt.Printf("📘 Loaded %d zone translations from %s\n", len(lookup), filepath.Base(lookupPath))
	}

	return &Window{
		Parser:     p,
		MapDir:     mapDir,
		ZoneLookup: lookup,
		Zoom:       1.0,
		
		BackgroundAlpha: 100,
		ShowTrail:       true,
		ShowLabels:      true,

		// Defaults
		PlayerMultX: -1.0, 
		PlayerMultY: -1.0,
		PlayerSwap:  false,
		
		Trail:      make([]Breadcrumb, 0),
		Width:      400,
		Height:     400,
	}
}

func (w *Window) Update() error {
	w.checkZoneChange()
	w.updateTrail()

	// --- TOGGLES ---
	if inpututil.IsKeyJustPressed(ebiten.KeyT) { w.ShowTrail = !w.ShowTrail }
	if inpututil.IsKeyJustPressed(ebiten.KeyC) { w.Trail = make([]Breadcrumb, 0) }
	if inpututil.IsKeyJustPressed(ebiten.KeyL) { w.ShowLabels = !w.ShowLabels }
	
	// 'K' Clears Corpse Marker (MANUAL OVERRIDE)
	if inpututil.IsKeyJustPressed(ebiten.KeyK) {
		w.Parser.CurrentState.HasCorpse = false
	}

	// [ / ] adjust opacity
	if ebiten.IsKeyPressed(ebiten.KeyLeftBracket) {
		if w.BackgroundAlpha > 0 { w.BackgroundAlpha -= 5 }
	}
	if ebiten.IsKeyPressed(ebiten.KeyRightBracket) {
		if w.BackgroundAlpha < 250 { w.BackgroundAlpha += 5 }
	}

	// --- PLAYER CALIBRATION ---
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) { w.PlayerMultX *= -1 }
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) { w.PlayerMultY *= -1 }
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) { w.PlayerSwap = !w.PlayerSwap }
	
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) { w.CenterOnMapBounds() }
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) { w.CenterOnPlayer() }

	// ZOOM
	_, dy := ebiten.Wheel()
	if dy != 0 {
		if dy > 0 { w.Zoom *= 1.1 } else { w.Zoom /= 1.1 }
	}

	// PAN
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		cx, cy := ebiten.CursorPosition()
		if !w.IsDragging {
			w.IsDragging = true
			w.DragStartX, w.DragStartY = cx, cy
		} else {
			w.OffsetX += float64(cx - w.DragStartX)
			w.OffsetY += float64(cy - w.DragStartY)
			w.DragStartX, w.DragStartY = cx, cy
		}
	} else {
		w.IsDragging = false
	}
	
	return nil
}

func (w *Window) updateTrail() {
	px, py := w.GetCalibratedPlayerPos()
	if len(w.Trail) == 0 {
		w.Trail = append(w.Trail, Breadcrumb{px, py})
		return
	}
	last := w.Trail[len(w.Trail)-1]
	dist := math.Sqrt(math.Pow(px-last.X, 2) + math.Pow(py-last.Y, 2))
	if dist > 10 {
		w.Trail = append(w.Trail, Breadcrumb{px, py})
		if len(w.Trail) > 500 { w.Trail = w.Trail[1:] }
	}
}

func (w *Window) GetCalibratedPlayerPos() (float64, float64) {
	rawX := w.Parser.CurrentState.X
	rawY := w.Parser.CurrentState.Y
	if w.PlayerSwap { rawX, rawY = rawY, rawX }
	return rawX * w.PlayerMultX, rawY * w.PlayerMultY
}

func (w *Window) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 0, 0, w.BackgroundAlpha})

	if w.CurrentMap == nil {
		ebitenutil.DebugPrint(screen, fmt.Sprintf("Waiting for Zone... (Last Seen: %s)", w.Parser.CurrentState.Zone))
		return
	}

	cx, cy := float64(w.Width)/2, float64(w.Height)/2
	
	// 1. Draw Lines
	for _, l := range w.CurrentMap.Lines {
		x1, y1 := w.mapToScreen(l.X1, l.Y1, cx, cy)
		x2, y2 := w.mapToScreen(l.X2, l.Y2, cx, cy)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, l.Color, true)
	}

	// 2. Draw Trail
	if w.ShowTrail {
		for _, b := range w.Trail {
			bx, by := w.mapToScreen(b.X, b.Y, cx, cy)
			vector.DrawFilledCircle(screen, float32(bx), float32(by), 1, color.RGBA{0, 255, 255, 100}, true)
		}
	}

	// 3. Draw Labels
	if w.ShowLabels {
		for _, p := range w.CurrentMap.Labels {
			px, py := w.mapToScreen(p.X, p.Y, cx, cy)
			if isZoneLabel(p.Text) {
				vector.DrawFilledCircle(screen, float32(px), float32(py), 4, color.RGBA{0, 255, 0, 255}, true)
				cleanText := formatZoneText(p.Text)
				ebitenutil.DebugPrintAt(screen, cleanText, int(px)+7, int(py)-5)
			} else {
				vector.DrawFilledCircle(screen, float32(px), float32(py), 2, p.Color, true)
				ebitenutil.DebugPrintAt(screen, p.Text, int(px)+4, int(py)-4)
			}
		}
	}

	// 4. Draw Corpse Run Line
	if w.Parser.CurrentState.HasCorpse {
		rawCX := w.Parser.CurrentState.CorpseX
		rawCY := w.Parser.CurrentState.CorpseY
		if w.PlayerSwap { rawCX, rawCY = rawCY, rawCX }
		
		calibratedCX := rawCX * w.PlayerMultX
		calibratedCY := rawCY * w.PlayerMultY

		cxScreen, cyScreen := w.mapToScreen(calibratedCX, calibratedCY, cx, cy)
		px, py := w.GetCalibratedPlayerPos()
		pxScreen, pyScreen := w.mapToScreen(px, py, cx, cy)

		// RED LINE to Corpse
		vector.StrokeLine(screen, float32(pxScreen), float32(pyScreen), float32(cxScreen), float32(cyScreen), 2, color.RGBA{255, 0, 0, 255}, true)
		
		// RED "X" at Corpse
		vector.StrokeLine(screen, float32(cxScreen)-5, float32(cyScreen)-5, float32(cxScreen)+5, float32(cyScreen)+5, 2, color.RGBA{255, 0, 0, 255}, true)
		vector.StrokeLine(screen, float32(cxScreen)-5, float32(cyScreen)+5, float32(cxScreen)+5, float32(cyScreen)-5, 2, color.RGBA{255, 0, 0, 255}, true)
		ebitenutil.DebugPrintAt(screen, "CORPSE", int(cxScreen)+5, int(cyScreen))
	}

	// 5. Draw Player Arrow
	worldPx, worldPy := w.GetCalibratedPlayerPos()
	px, py := w.mapToScreen(worldPx, worldPy, cx, cy)
	drawPlayerArrow(screen, px, py, w.Parser.CurrentState.Heading, w.PlayerMultX, w.PlayerMultY)

	// UI
	trailStatus := "ON"
	if !w.ShowTrail { trailStatus = "OFF" }
	labelStatus := "ON"
	if !w.ShowLabels { labelStatus = "OFF" }
	
	// Dynamic Corpse Status
	corpseStatus := ""
	if w.Parser.CurrentState.HasCorpse { 
		corpseStatus = "| [K] CLEAR CORPSE" 
	}

	status := fmt.Sprintf("Zone: %s | Alpha: %d\n[T] Trail:%s | [L] Labels:%s %s\n[Space] Center", 
		w.CurrentMap.Name, w.BackgroundAlpha, trailStatus, labelStatus, corpseStatus)
	ebitenutil.DebugPrint(screen, status)

	mx, my := ebiten.CursorPosition()
	mapMX, mapMY := w.screenToMap(float64(mx), float64(my), cx, cy)
	finalMX := mapMX * w.PlayerMultX
	finalMY := mapMY * w.PlayerMultY
	cursorText := fmt.Sprintf("Cursor: %.0f, %.0f", finalMX, finalMY)
	ebitenutil.DebugPrintAt(screen, cursorText, w.Width - 140, 10)
}

func drawPlayerArrow(screen *ebiten.Image, cx, cy, heading, multX, multY float64) {
	size := 8.0
	p1x, p1y := size, 0.0
	p2x, p2y := -size/2, -size/2
	p3x, p3y := -size/2, size/2

	rotate := func(x, y, angle float64) (float64, float64) {
		sin, cos := math.Sincos(angle)
		return x*cos - y*sin, x*sin + y*cos
	}

	rx1, ry1 := rotate(p1x, p1y, heading)
	rx2, ry2 := rotate(p2x, p2y, heading)
	rx3, ry3 := rotate(p3x, p3y, heading)

	rx1 *= multX; ry1 *= multY
	rx2 *= multX; ry2 *= multY
	rx3 *= multX; ry3 *= multY

	vector.StrokeLine(screen, float32(cx+rx2), float32(cy+ry2), float32(cx+rx1), float32(cy+ry1), 2, color.RGBA{255, 0, 0, 255}, true)
	vector.StrokeLine(screen, float32(cx+rx1), float32(cy+ry1), float32(cx+rx3), float32(cy+ry3), 2, color.RGBA{255, 0, 0, 255}, true)
	vector.StrokeLine(screen, float32(cx+rx3), float32(cy+ry3), float32(cx+rx2), float32(cy+ry2), 2, color.RGBA{255, 0, 0, 255}, true)
}

func (w *Window) mapToScreen(wx, wy, cx, cy float64) (float64, float64) {
	screenX := (wx * 1.0 * w.Zoom) + w.OffsetX + cx
	screenY := (wy * 1.0 * w.Zoom) + w.OffsetY + cy
	return screenX, screenY
}

func (w *Window) screenToMap(sx, sy, cx, cy float64) (float64, float64) {
	wx := (sx - cx - w.OffsetX) / w.Zoom
	wy := (sy - cy - w.OffsetY) / w.Zoom
	return wx, wy
}

func (w *Window) checkZoneChange() {
	newZoneName := w.Parser.CurrentState.Zone
	if w.CurrentMap != nil && w.CurrentMap.Name == newZoneName { return }
	if newZoneName == "" || newZoneName == "Unknown" { return }

	shortName, exists := w.ZoneLookup[strings.ToLower(newZoneName)]
	if !exists { shortName = newZoneName }

	w.Trail = make([]Breadcrumb, 0)
	fmt.Printf("🗺️ Zone Change Detected: %s -> Loading %s...\n", newZoneName, shortName)
	w.LoadMap(shortName, newZoneName)
}

func (w *Window) LoadMap(shortName, longName string) {
	m, err := maps.LoadZone(w.MapDir, shortName)
	if err != nil {
		fmt.Printf("FAILED to load map: %v\n", err)
		m = &maps.ZoneMap{Name: longName}
	} else {
		fmt.Printf("SUCCESS. Loaded map with %d lines, %d labels.\n", len(m.Lines), len(m.Labels))
	}
	m.Name = longName
	w.CurrentMap = m
	w.CenterOnPlayer()
}

func (w *Window) CenterOnPlayer() {
	px, py := w.GetCalibratedPlayerPos()
	w.OffsetX = -(px * w.Zoom)
	w.OffsetY = -(py * w.Zoom)
}

func (w *Window) CenterOnMapBounds() {
	if w.CurrentMap == nil { return }
	midX := (w.CurrentMap.MinX + w.CurrentMap.MaxX) / 2
	midY := (w.CurrentMap.MinY + w.CurrentMap.MaxY) / 2
	w.OffsetX = -(midX * w.Zoom)
	w.OffsetY = -(midY * w.Zoom)
}

func (w *Window) Layout(outsideWidth, outsideHeight int) (int, int) {
	w.Width = outsideWidth
	w.Height = outsideHeight
	return w.Width, w.Height
}

// --- HELPER FUNCTIONS ---

func isZoneLabel(text string) bool {
	t := strings.ToLower(text)
	return strings.HasPrefix(t, "to_") || strings.Contains(t, "zone line")
}

func formatZoneText(text string) string {
	return strings.ReplaceAll(text, "_", " ")
}