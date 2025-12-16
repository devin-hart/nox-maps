package ui

import (
	"fmt"
	"image/color"
	"math"

	"github.com/devin-hart/nox-maps/internal/maps"
	"github.com/devin-hart/nox-maps/internal/parser"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

type Window struct {
	Width, Height int
	Title         string

	// Data Sources
	LogReader     *parser.Engine
	MapData       *maps.ZoneMap
	MapDir        string
	MapConfigPath string
	CurrentZone   string

	// Viewport State
	CamX, CamY float64
	Zoom       float64

	// Display Options
	Opacity     float64
	ShowLabels  bool
	Breadcrumbs []BreadcrumbPoint

	// Input State
	lastMouseX      int
	lastMouseY      int
	lastBracketKey  bool
	lastRBracketKey bool
	lastLKey        bool
	lastCKey        bool
}

type BreadcrumbPoint struct {
	X, Y float64
}

func NewWindow(engine *parser.Engine, mapDir string, mapConfigPath string) *Window {
	return &Window{
		Width:         1280,
		Height:        720,
		Title:         "Nox Maps",
		LogReader:     engine,
		MapDir:        mapDir,
		MapConfigPath: mapConfigPath,
		Zoom:          1.0,
		Opacity:       1.0,
		ShowLabels:    true,
		Breadcrumbs:   make([]BreadcrumbPoint, 0),
	}
}

func (w *Window) Init() error {
	ebiten.SetWindowTitle(w.Title)
	ebiten.SetWindowSize(w.Width, w.Height)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenTransparent(true)

	maps.LoadZoneConfig(w.MapConfigPath)
	return nil
}

func (w *Window) Update() error {
	// 1. MOUSE ZOOM (Wheel)
	_, dy := ebiten.Wheel()
	if dy > 0 {
		w.Zoom *= 1.1
	} else if dy < 0 {
		w.Zoom /= 1.1
	}

	// 2. MOUSE PAN (Right Click)
	mx, my := ebiten.CursorPosition()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		dx := float64(mx - w.lastMouseX)
		dy := float64(my - w.lastMouseY)
		
		// Move Camera OPPOSITE to mouse drag to simulate "grabbing" the map
		w.CamX -= dx / w.Zoom
		w.CamY -= dy / w.Zoom
	}
	w.lastMouseX = mx
	w.lastMouseY = my

	// 3. KEYBOARD PAN
	moveSpeed := 10.0 / w.Zoom
	if ebiten.IsKeyPressed(ebiten.KeyW) { w.CamY -= moveSpeed } // Up moves camera up (decreases Y)
	if ebiten.IsKeyPressed(ebiten.KeyS) { w.CamY += moveSpeed }
	if ebiten.IsKeyPressed(ebiten.KeyA) { w.CamX -= moveSpeed }
	if ebiten.IsKeyPressed(ebiten.KeyD) { w.CamX += moveSpeed }

	// 4. CENTER ON PLAYER (Spacebar)
	if ebiten.IsKeyPressed(ebiten.KeySpace) && w.LogReader != nil {
		w.CamX = w.LogReader.CurrentState.X
		w.CamY = w.LogReader.CurrentState.Y
	}

	// 5. OPACITY CONTROLS ([ and ])
	bracketPressed := ebiten.IsKeyPressed(ebiten.KeyBracketLeft)
	if bracketPressed && !w.lastBracketKey {
		w.Opacity -= 0.1
		if w.Opacity < 0.1 { w.Opacity = 0.1 }
	}
	w.lastBracketKey = bracketPressed

	rBracketPressed := ebiten.IsKeyPressed(ebiten.KeyBracketRight)
	if rBracketPressed && !w.lastRBracketKey {
		w.Opacity += 0.1
		if w.Opacity > 1.0 { w.Opacity = 1.0 }
	}
	w.lastRBracketKey = rBracketPressed

	// 6. TOGGLE LABELS (L key)
	lPressed := ebiten.IsKeyPressed(ebiten.KeyL)
	if lPressed && !w.lastLKey {
		w.ShowLabels = !w.ShowLabels
	}
	w.lastLKey = lPressed

	// 7. CLEAR BREADCRUMBS (C key)
	cPressed := ebiten.IsKeyPressed(ebiten.KeyC)
	if cPressed && !w.lastCKey {
		w.Breadcrumbs = w.Breadcrumbs[:0]
	}
	w.lastCKey = cPressed

	// 8. BREADCRUMB TRACKING
	// Add a breadcrumb every ~2 seconds when player moves
	if w.LogReader != nil {
		shouldAddBreadcrumb := false
		if len(w.Breadcrumbs) == 0 {
			shouldAddBreadcrumb = true
		} else {
			lastBC := w.Breadcrumbs[len(w.Breadcrumbs)-1]
			dx := w.LogReader.CurrentState.X - lastBC.X
			dy := w.LogReader.CurrentState.Y - lastBC.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			// Add breadcrumb if moved more than 50 units
			if dist > 50 {
				shouldAddBreadcrumb = true
			}
		}

		if shouldAddBreadcrumb {
			w.Breadcrumbs = append(w.Breadcrumbs, BreadcrumbPoint{
				X: w.LogReader.CurrentState.X,
				Y: w.LogReader.CurrentState.Y,
			})
			// Limit to last 500 breadcrumbs
			if len(w.Breadcrumbs) > 500 {
				w.Breadcrumbs = w.Breadcrumbs[1:]
			}
		}
	}

	// 9. ZONE CHANGE DETECTION
	if w.LogReader != nil && w.LogReader.CurrentState.Zone != w.CurrentZone {
		w.CurrentZone = w.LogReader.CurrentState.Zone
		w.loadMapForZone(w.CurrentZone)
		w.Breadcrumbs = w.Breadcrumbs[:0] // Clear breadcrumbs when changing zones
	}
	return nil
}

func (w *Window) loadMapForZone(zoneName string) {
	fmt.Printf("\n🗺️  Loading zone: '%s'\n", zoneName)
	fileCode := maps.GetZoneFileName(zoneName)
	if fileCode == "" {
		fileCode = zoneName
		fmt.Printf("  No mapping found, using zone name as filename\n")
	} else {
		fmt.Printf("  Mapped to file: '%s'\n", fileCode)
	}

	data, err := maps.LoadZone(w.MapDir, fileCode)
	if err != nil {
		fmt.Printf("❌ Error loading map %s: %v\n", zoneName, err)
		w.MapData = nil
	} else {
		w.MapData = data
		fmt.Printf("✅ Map loaded: %d lines, %d labels\n", len(data.Lines), len(data.Labels))
		fmt.Printf("  Bounds: X[%.0f to %.0f] Y[%.0f to %.0f]\n",
			data.MinX, data.MaxX, data.MinY, data.MaxY)

		// Auto-center camera
		w.CamX = (data.MinX + data.MaxX) / 2
		w.CamY = (data.MinY + data.MaxY) / 2
		fmt.Printf("  Camera centered at: (%.1f, %.1f)\n", w.CamX, w.CamY)
	}
}

func (w *Window) Draw(screen *ebiten.Image) {
	// Create offscreen image for all map content
	offscreen := ebiten.NewImage(w.Width, w.Height)
	offscreen.Fill(color.Black)

	cx, cy := float64(w.Width)/2, float64(w.Height)/2

	if w.MapData != nil {
		// DRAW LINES
		for _, line := range w.MapData.Lines {
			x1 := (line.X1 - w.CamX) * w.Zoom + cx
			y1 := (line.Y1 - w.CamY) * w.Zoom + cy
			x2 := (line.X2 - w.CamX) * w.Zoom + cx
			y2 := (line.Y2 - w.CamY) * w.Zoom + cy
			ebitenutil.DrawLine(offscreen, x1, y1, x2, y2, line.Color)
		}

		// DRAW LABELS (if enabled)
		if w.ShowLabels {
			for _, lbl := range w.MapData.Labels {
				lx := (lbl.X - w.CamX) * w.Zoom + cx
				ly := (lbl.Y - w.CamY) * w.Zoom + cy

				if lx > -50 && lx < float64(w.Width)+50 && ly > -50 && ly < float64(w.Height)+50 {
					text.Draw(offscreen, lbl.Text, basicfont.Face7x13, int(lx), int(ly), lbl.Color)
				}
			}
		}

		// DRAW BREADCRUMBS
		breadcrumbColor := color.RGBA{255, 255, 0, 200}
		for _, bc := range w.Breadcrumbs {
			bx := (bc.X - w.CamX) * w.Zoom + cx
			by := (bc.Y - w.CamY) * w.Zoom + cy
			ebitenutil.DrawLine(offscreen, bx, by, bx+1, by+1, breadcrumbColor)
		}
	}

	// DRAW PLAYER ARROW
	if w.LogReader != nil {
		w.drawPlayerArrow(offscreen, cx, cy)
	}

	// DRAW UI / DEBUG
	w.drawUI(offscreen)

	// Apply opacity to entire screen and enable filtering for anti-aliasing
	opts := &ebiten.DrawImageOptions{}
	opts.ColorScale.ScaleAlpha(float32(w.Opacity))
	opts.Filter = ebiten.FilterLinear
	screen.DrawImage(offscreen, opts)
}

func (w *Window) drawPlayerArrow(screen *ebiten.Image, cx, cy float64) {
	s := w.LogReader.CurrentState
	
	// Convert Player World Pos to Screen Pos
	px := (s.X - w.CamX) * w.Zoom + cx
	py := (s.Y - w.CamY) * w.Zoom + cy

	// Heading
	angle := s.Heading 

	size := 10.0 * w.Zoom
	if size < 8 { size = 8 }
	if size > 25 { size = 25 }

	// Calculate arrow points
	x1 := px + math.Cos(angle)*size
	y1 := py + math.Sin(angle)*size 

	x2 := px + math.Cos(angle + 2.6)*size
	y2 := py + math.Sin(angle + 2.6)*size

	x3 := px + math.Cos(angle - 2.6)*size
	y3 := py + math.Sin(angle - 2.6)*size

	c := color.RGBA{0, 255, 0, 255}
	ebitenutil.DrawLine(screen, x1, y1, x2, y2, c)
	ebitenutil.DrawLine(screen, x2, y2, x3, y3, c)
	ebitenutil.DrawLine(screen, x3, y3, x1, y1, c)
}

func (w *Window) drawUI(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	cx, cy := float64(w.Width)/2, float64(w.Height)/2

	// Reverse transform: Screen -> World
	worldX := (float64(mx) - cx) / w.Zoom + w.CamX
	worldY := (float64(my) - cy) / w.Zoom + w.CamY

	var mapInfo string
	if w.MapData != nil {
		mapInfo = fmt.Sprintf("\nMap Bounds: X[%.0f to %.0f] Y[%.0f to %.0f]",
			w.MapData.MinX, w.MapData.MaxX, w.MapData.MinY, w.MapData.MaxY)
	}

	labelsStatus := "ON"
	if !w.ShowLabels {
		labelsStatus = "OFF"
	}

	msg := fmt.Sprintf("Zone: %s | Zoom: %.2f | Opacity: %.0f%%\nCam: %.1f, %.1f\nMouse: %.1f, %.1f\nPlayer: %.1f, %.1f%s\n[SPACE] Center | [L] Labels:%s | [ ] Opacity | [C] Clear Breadcrumbs: %d",
		w.CurrentZone, w.Zoom, w.Opacity*100, w.CamX, w.CamY, worldX, worldY,
		w.LogReader.CurrentState.X, w.LogReader.CurrentState.Y, mapInfo, labelsStatus, len(w.Breadcrumbs))

	ebitenutil.DebugPrint(screen, msg)
}

func (w *Window) Layout(outsideWidth, outsideHeight int) (int, int) {
	w.Width = outsideWidth
	w.Height = outsideHeight
	return outsideWidth, outsideHeight
}