package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"

	"github.com/devin-hart/nox-maps/internal/config"
	"github.com/devin-hart/nox-maps/internal/maps"
	"github.com/devin-hart/nox-maps/internal/parser"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/ncruces/zenity"
	"golang.org/x/image/font/basicfont"
)

var whiteImage = ebiten.NewImage(3, 3)

func init() {
	whiteImage.Fill(color.White)
}

type Window struct {
	Width, Height int
	Title         string

	// Data Sources
	LogReader     *parser.Engine
	MapData       *maps.ZoneMap
	MapDir        string
	MapConfigPath string
	CurrentZone   string
	Config        *config.Config

	// Viewport State
	CamX, CamY float64
	Zoom       float64

	// Display Options
	Opacity         float64
	LabelMode       int // 0 = all, 1 = custom+zone lines, 2 = zone lines only, 3 = none
	ShowBreadcrumbs bool
	Breadcrumbs     []BreadcrumbPoint

	// Z-Level Filtering
	ZLevelMode      int     // 0 = off, 1 = auto, 2 = manual
	ZLevelManual    float64 // Manual Z level when in manual mode
	ZLevelRange     float64 // +/- range to show around Z level

	// Input State
	lastMouseX        int
	lastMouseY        int
	lastMousePressed  bool
	lastMinusKey      bool
	lastEqualsKey     bool
	lastLKey          bool
	lastBKey          bool
	lastCKey          bool
	lastKKey          bool
	lastZKey          bool
	lastPageUpKey   bool
	lastPageDownKey bool
	lastInsertKey   bool
	lastDeleteKey   bool
	lastHomeKey     bool
	lastMKey        bool

	// Menu State
	openMenu       string // "File", "View", "Help", or ""
	openSubmenu    int    // Index of menu item with open submenu (-1 if none)
	menuBarHeight  int
	showInfo       bool   // Show info panel

	// Marker State
	placingMarker bool
	markerColor   string
	markerShape   string
	ShowMarkers   bool
	lastRKey      bool
	dialogOpen    bool // Prevents re-entry while zenity dialog is open
}

type BreadcrumbPoint struct {
	X, Y float64
}

func NewWindow(engine *parser.Engine, mapDir string, mapConfigPath string, cfg *config.Config) *Window {
	return &Window{
		Width:           1280,
		Height:          720,
		Title:           "Nox Maps",
		LogReader:       engine,
		MapDir:          mapDir,
		MapConfigPath:   mapConfigPath,
		Config:          cfg,
		Zoom:            1.0,
		Opacity:         1.0,
		LabelMode:       2, // Default to zone lines only
		ShowBreadcrumbs: true,
		Breadcrumbs:     make([]BreadcrumbPoint, 0),
		ZLevelMode:      0,    // Default to off (0=off, 1=auto, 2=manual)
		ZLevelManual:    0.0,
		ZLevelRange:     50.0, // Show +/- 50 units
		menuBarHeight:   24,
		openMenu:        "",
		openSubmenu:     -1,
		showInfo:        true, // Show info panel by default
		placingMarker:   false,
		markerColor:     "red",
		markerShape:     "circle",
		ShowMarkers:     true, // Show markers by default
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

	// 2. MOUSE INPUT
	mx, my := ebiten.CursorPosition()
	cx, cy := float64(w.Width)/2, float64(w.Height)/2

	// Convert screen coordinates to world coordinates
	worldX := (float64(mx) - cx) / w.Zoom + w.CamX
	worldY := (float64(my) - cy) / w.Zoom + w.CamY

	// Left-click handling
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && !w.lastMousePressed && !w.dialogOpen {
		// Only handle clicks below menu bar
		if my > w.menuBarHeight {
			if w.placingMarker {
				// Place new marker
				w.placeMarker(worldX, worldY)
			} else {
				// Check if clicking on existing marker to edit label
				w.editMarkerAt(worldX, worldY)
			}
		}
	}

	// Right-click handling
	rightPressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	markerRemoved := false
	if rightPressed && !w.lastMousePressed {
		// Check if right-clicking on a marker to delete it
		if my > w.menuBarHeight {
			markerRemoved = w.removeMarkerAt(worldX, worldY)
		}
	}

	// Pan the map when right button is held (unless we just removed a marker)
	if rightPressed && !markerRemoved {
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

	// 5. OPACITY CONTROLS (- and =)
	minusPressed := ebiten.IsKeyPressed(ebiten.KeyMinus)
	if minusPressed && !w.lastMinusKey {
		w.Opacity -= 0.1
		if w.Opacity < 0.1 { w.Opacity = 0.1 }
	}
	w.lastMinusKey = minusPressed

	equalsPressed := ebiten.IsKeyPressed(ebiten.KeyEqual)
	if equalsPressed && !w.lastEqualsKey {
		w.Opacity += 0.1
		if w.Opacity > 1.0 { w.Opacity = 1.0 }
	}
	w.lastEqualsKey = equalsPressed

	// 6. CYCLE LABEL MODE (L key)
	// 0 = all, 1 = custom+zone lines, 2 = zone lines only, 3 = none
	lPressed := ebiten.IsKeyPressed(ebiten.KeyL)
	if lPressed && !w.lastLKey {
		w.LabelMode = (w.LabelMode + 1) % 4
	}
	w.lastLKey = lPressed

	// 7. TOGGLE BREADCRUMBS (B key)
	bPressed := ebiten.IsKeyPressed(ebiten.KeyB)
	if bPressed && !w.lastBKey {
		w.ShowBreadcrumbs = !w.ShowBreadcrumbs
	}
	w.lastBKey = bPressed

	// 8. CLEAR BREADCRUMBS (C key)
	cPressed := ebiten.IsKeyPressed(ebiten.KeyC)
	if cPressed && !w.lastCKey {
		w.Breadcrumbs = w.Breadcrumbs[:0]
	}
	w.lastCKey = cPressed

	// 9. CLEAR CORPSE (K key)
	kPressed := ebiten.IsKeyPressed(ebiten.KeyK)
	if kPressed && !w.lastKKey && w.LogReader != nil {
		w.LogReader.CurrentState.HasCorpse = false
	}
	w.lastKKey = kPressed

	// 10. CYCLE Z-LEVEL MODE (Z key)
	// 0 = off, 1 = auto, 2 = manual
	zPressed := ebiten.IsKeyPressed(ebiten.KeyZ)
	if zPressed && !w.lastZKey {
		w.ZLevelMode = (w.ZLevelMode + 1) % 3
		// When switching to manual, set manual level to current player Z
		if w.ZLevelMode == 2 && w.LogReader != nil {
			w.ZLevelManual = w.LogReader.CurrentState.Z
		}
	}
	w.lastZKey = zPressed

	// 11. MANUAL Z-LEVEL ADJUSTMENT (PageUp/PageDown)
	pageUpPressed := ebiten.IsKeyPressed(ebiten.KeyPageUp)
	if pageUpPressed && !w.lastPageUpKey {
		w.ZLevelManual += 10.0
		w.ZLevelMode = 2 // Switch to manual mode
	}
	w.lastPageUpKey = pageUpPressed

	pageDownPressed := ebiten.IsKeyPressed(ebiten.KeyPageDown)
	if pageDownPressed && !w.lastPageDownKey {
		w.ZLevelManual -= 10.0
		w.ZLevelMode = 2 // Switch to manual mode
	}
	w.lastPageDownKey = pageDownPressed

	// 12. Z-LEVEL RANGE ADJUSTMENT (Insert and Delete)
	insertPressed := ebiten.IsKeyPressed(ebiten.KeyInsert)
	if insertPressed && !w.lastInsertKey {
		w.ZLevelRange += 10.0
		if w.ZLevelRange > 200.0 {
			w.ZLevelRange = 200.0 // Maximum range
		}
	}
	w.lastInsertKey = insertPressed

	deletePressed := ebiten.IsKeyPressed(ebiten.KeyDelete)
	if deletePressed && !w.lastDeleteKey {
		w.ZLevelRange -= 10.0
		if w.ZLevelRange < 10.0 {
			w.ZLevelRange = 10.0 // Minimum range
		}
	}
	w.lastDeleteKey = deletePressed

	// 13. RE-FIT ZOOM (Home key)
	homePressed := ebiten.IsKeyPressed(ebiten.KeyHome)
	if homePressed && !w.lastHomeKey && w.MapData != nil {
		w.refitZoom()
	}
	w.lastHomeKey = homePressed

	// 14. MARKER PLACEMENT (M key to toggle mode)
	mPressed := ebiten.IsKeyPressed(ebiten.KeyM)
	if mPressed && !w.lastMKey {
		w.placingMarker = !w.placingMarker
		if w.placingMarker {
			fmt.Println("📍 Marker placement mode ON - Left-click to place marker")
		} else {
			fmt.Println("📍 Marker placement mode OFF")
		}
	}
	w.lastMKey = mPressed

	// 15. TOGGLE MARKER VISIBILITY (R key)
	rPressed := ebiten.IsKeyPressed(ebiten.KeyR)
	if rPressed && !w.lastRKey {
		w.ShowMarkers = !w.ShowMarkers
		if w.ShowMarkers {
			fmt.Println("📍 Markers visible")
		} else {
			fmt.Println("📍 Markers hidden")
		}
	}
	w.lastRKey = rPressed

	// 16. BREADCRUMB TRACKING
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

	// 11. ZONE CHANGE DETECTION
	if w.LogReader != nil && w.LogReader.CurrentState.Zone != w.CurrentZone {
		w.CurrentZone = w.LogReader.CurrentState.Zone
		w.loadMapForZone(w.CurrentZone)
		w.Breadcrumbs = w.Breadcrumbs[:0] // Clear breadcrumbs when changing zones
		// Note: Corpse marker persists across zone changes intentionally
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

		// Auto-center camera and zoom to fit
		// If Z-level filtering is enabled, calculate bounds for visible lines only
		var minX, maxX, minY, maxY float64

		if w.ZLevelMode > 0 && w.LogReader != nil {
			// Calculate bounds for current Z-level
			var activeZ float64
			if w.ZLevelMode == 1 {
				activeZ = w.LogReader.CurrentState.Z
			} else {
				activeZ = w.ZLevelManual
			}

			minX, maxX = 99999.0, -99999.0
			minY, maxY = 99999.0, -99999.0
			foundVisibleLines := false

			for _, line := range data.Lines {
				z1InRange := math.Abs(line.Z1-activeZ) <= w.ZLevelRange
				z2InRange := math.Abs(line.Z2-activeZ) <= w.ZLevelRange
				if z1InRange || z2InRange {
					if line.X1 < minX { minX = line.X1 }
					if line.X1 > maxX { maxX = line.X1 }
					if line.Y1 < minY { minY = line.Y1 }
					if line.Y1 > maxY { maxY = line.Y1 }
					if line.X2 < minX { minX = line.X2 }
					if line.X2 > maxX { maxX = line.X2 }
					if line.Y2 < minY { minY = line.Y2 }
					if line.Y2 > maxY { maxY = line.Y2 }
					foundVisibleLines = true
				}
			}

			// If no visible lines, fall back to full map bounds
			if !foundVisibleLines {
				minX, maxX = data.MinX, data.MaxX
				minY, maxY = data.MinY, data.MaxY
			}
		} else {
			// Use full map bounds when Z-filtering is off
			minX, maxX = data.MinX, data.MaxX
			minY, maxY = data.MinY, data.MaxY
		}

		w.CamX = (minX + maxX) / 2
		w.CamY = (minY + maxY) / 2

		// Calculate zoom to fit visible geometry in window with some padding
		mapWidth := maxX - minX
		mapHeight := maxY - minY

		// Add 10% padding so map doesn't touch edges
		zoomX := float64(w.Width) * 0.9 / mapWidth
		zoomY := float64(w.Height) * 0.9 / mapHeight

		// Use the smaller zoom to ensure entire map fits
		if zoomX < zoomY {
			w.Zoom = zoomX
		} else {
			w.Zoom = zoomY
		}

		fmt.Printf("  Camera centered at: (%.1f, %.1f), Zoom: %.3f\n", w.CamX, w.CamY, w.Zoom)
	}
}

func (w *Window) getMarkerColor(colorName string) color.RGBA {
	switch colorName {
	case "red":
		return color.RGBA{255, 0, 0, 255}
	case "blue":
		return color.RGBA{0, 100, 255, 255}
	case "green":
		return color.RGBA{0, 255, 0, 255}
	case "yellow":
		return color.RGBA{255, 255, 0, 255}
	case "purple":
		return color.RGBA{200, 0, 255, 255}
	default:
		return color.RGBA{255, 0, 0, 255} // Default to red
	}
}

func (w *Window) drawMarkerShape(screen *ebiten.Image, mx, my float32, shape string, markerColor color.RGBA) {
	size := float32(8.0)
	blackOutline := color.RGBA{0, 0, 0, 255}

	// Default to circle if shape is empty or unknown
	if shape == "" {
		shape = "circle"
	}

	switch shape {
	case "circle":
		vector.DrawFilledCircle(screen, mx, my, size, markerColor, true)
		vector.StrokeCircle(screen, mx, my, size, 2.0, blackOutline, true)

	case "square":
		// Draw filled square
		vector.DrawFilledRect(screen, mx-size, my-size, size*2, size*2, markerColor, true)
		// Draw outline
		vector.StrokeRect(screen, mx-size, my-size, size*2, size*2, 2.0, blackOutline, true)

	case "triangle":
		// Draw upward-pointing triangle
		var path vector.Path
		path.MoveTo(mx, my-size)           // Top point
		path.LineTo(mx+size, my+size)      // Bottom right
		path.LineTo(mx-size, my+size)      // Bottom left
		path.Close()

		vertices, indices := path.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vertices {
			vertices[i].ColorR = float32(markerColor.R) / 255
			vertices[i].ColorG = float32(markerColor.G) / 255
			vertices[i].ColorB = float32(markerColor.B) / 255
			vertices[i].ColorA = float32(markerColor.A) / 255
		}
		screen.DrawTriangles(vertices, indices, whiteImage, &ebiten.DrawTrianglesOptions{
			AntiAlias: true,
		})

		// Draw outline
		vector.StrokeLine(screen, mx, my-size, mx+size, my+size, 2.0, blackOutline, true)
		vector.StrokeLine(screen, mx+size, my+size, mx-size, my+size, 2.0, blackOutline, true)
		vector.StrokeLine(screen, mx-size, my+size, mx, my-size, 2.0, blackOutline, true)

	case "diamond":
		// Draw diamond (rotated square)
		var path vector.Path
		path.MoveTo(mx, my-size)       // Top
		path.LineTo(mx+size, my)       // Right
		path.LineTo(mx, my+size)       // Bottom
		path.LineTo(mx-size, my)       // Left
		path.Close()

		vertices, indices := path.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vertices {
			vertices[i].ColorR = float32(markerColor.R) / 255
			vertices[i].ColorG = float32(markerColor.G) / 255
			vertices[i].ColorB = float32(markerColor.B) / 255
			vertices[i].ColorA = float32(markerColor.A) / 255
		}
		screen.DrawTriangles(vertices, indices, whiteImage, &ebiten.DrawTrianglesOptions{
			AntiAlias: true,
		})

		// Draw outline
		vector.StrokeLine(screen, mx, my-size, mx+size, my, 2.0, blackOutline, true)
		vector.StrokeLine(screen, mx+size, my, mx, my+size, 2.0, blackOutline, true)
		vector.StrokeLine(screen, mx, my+size, mx-size, my, 2.0, blackOutline, true)
		vector.StrokeLine(screen, mx-size, my, mx, my-size, 2.0, blackOutline, true)

	case "star":
		// Draw 5-pointed star
		var path vector.Path
		outerRadius := size
		innerRadius := size * 0.4

		for i := 0; i < 10; i++ {
			angle := float64(i) * math.Pi / 5.0 - math.Pi/2.0 // Start from top
			radius := outerRadius
			if i%2 == 1 {
				radius = innerRadius
			}
			x := mx + float32(math.Cos(angle)*float64(radius))
			y := my + float32(math.Sin(angle)*float64(radius))

			if i == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		path.Close()

		vertices, indices := path.AppendVerticesAndIndicesForFilling(nil, nil)
		for i := range vertices {
			vertices[i].ColorR = float32(markerColor.R) / 255
			vertices[i].ColorG = float32(markerColor.G) / 255
			vertices[i].ColorB = float32(markerColor.B) / 255
			vertices[i].ColorA = float32(markerColor.A) / 255
		}
		screen.DrawTriangles(vertices, indices, whiteImage, &ebiten.DrawTrianglesOptions{
			AntiAlias: true,
		})

		// Draw outline by connecting all points
		for i := 0; i < 10; i++ {
			angle1 := float64(i) * math.Pi / 5.0 - math.Pi/2.0
			angle2 := float64((i+1)%10) * math.Pi / 5.0 - math.Pi/2.0
			radius1 := outerRadius
			if i%2 == 1 {
				radius1 = innerRadius
			}
			radius2 := outerRadius
			if (i+1)%2 == 1 {
				radius2 = innerRadius
			}
			x1 := mx + float32(math.Cos(angle1)*float64(radius1))
			y1 := my + float32(math.Sin(angle1)*float64(radius1))
			x2 := mx + float32(math.Cos(angle2)*float64(radius2))
			y2 := my + float32(math.Sin(angle2)*float64(radius2))
			vector.StrokeLine(screen, x1, y1, x2, y2, 2.0, blackOutline, true)
		}

	default:
		// Fallback to circle
		vector.DrawFilledCircle(screen, mx, my, size, markerColor, true)
		vector.StrokeCircle(screen, mx, my, size, 2.0, blackOutline, true)
	}
}

func (w *Window) placeMarker(worldX, worldY float64) {
	if w.CurrentZone == "" {
		fmt.Println("⚠️  Cannot place marker: no active zone")
		return
	}

	// Prompt for marker label
	markerCount := len(w.Config.Markers[w.CurrentZone]) + 1
	defaultLabel := fmt.Sprintf("Marker %d", markerCount)

	w.dialogOpen = true
	label, err := zenity.Entry(
		"Enter marker label:",
		zenity.Title("New Marker"),
		zenity.EntryText(defaultLabel),
	)
	w.dialogOpen = false
	w.lastMousePressed = true // Prevent re-triggering on dialog close

	// If user cancelled or error occurred, do nothing
	if err != nil {
		fmt.Println("📍 Marker placement cancelled")
		w.placingMarker = false
		return
	}

	// Use default if empty
	if label == "" {
		label = defaultLabel
	}

	marker := config.Marker{
		X:     worldX,
		Y:     worldY,
		Label: label,
		Color: w.markerColor,
		Shape: w.markerShape,
	}

	// Add marker to config
	w.Config.Markers[w.CurrentZone] = append(w.Config.Markers[w.CurrentZone], marker)

	// Save to disk
	if err := w.Config.Save(); err != nil {
		fmt.Printf("❌ Error saving marker: %v\n", err)
	} else {
		fmt.Printf("📍 Marker placed: '%s' at (%.1f, %.1f) in %s\n", label, worldX, worldY, w.CurrentZone)
	}

	// Exit placement mode after placing marker
	w.placingMarker = false
}

func (w *Window) removeMarkerAt(worldX, worldY float64) bool {
	if w.CurrentZone == "" {
		return false
	}

	markers, ok := w.Config.Markers[w.CurrentZone]
	if !ok || len(markers) == 0 {
		return false
	}

	// Check if click is within range of any marker
	// Use a fixed click radius of 15 units in world space
	clickRadius := 15.0 / w.Zoom

	for i, marker := range markers {
		dx := worldX - marker.X
		dy := worldY - marker.Y
		distance := math.Sqrt(dx*dx + dy*dy)

		if distance <= clickRadius {
			// Confirm deletion
			w.dialogOpen = true
			err := zenity.Question(
				fmt.Sprintf("Delete marker '%s'?", marker.Label),
				zenity.Title("Confirm Delete"),
				zenity.OKLabel("Delete"),
				zenity.CancelLabel("Cancel"),
			)
			w.dialogOpen = false
			w.lastMousePressed = true // Prevent re-triggering

			if err != nil {
				// User cancelled
				return false
			}

			// Remove this marker
			w.Config.Markers[w.CurrentZone] = append(markers[:i], markers[i+1:]...)

			// Remove the zone entry if no markers left
			if len(w.Config.Markers[w.CurrentZone]) == 0 {
				delete(w.Config.Markers, w.CurrentZone)
			}

			// Save to disk
			if err := w.Config.Save(); err != nil {
				fmt.Printf("❌ Error removing marker: %v\n", err)
			} else {
				fmt.Printf("🗑️  Marker removed: '%s' from %s\n", marker.Label, w.CurrentZone)
			}

			return true
		}
	}

	return false
}

func (w *Window) clearAllMarkers() {
	if w.CurrentZone == "" {
		return
	}

	markers, ok := w.Config.Markers[w.CurrentZone]
	if !ok || len(markers) == 0 {
		w.dialogOpen = true
		zenity.Info(
			"No markers to delete in this zone.",
			zenity.Title("No Markers"),
		)
		w.dialogOpen = false
		w.lastMousePressed = true
		return
	}

	// Confirm deletion
	w.dialogOpen = true
	err := zenity.Question(
		fmt.Sprintf("Delete all %d markers in %s?", len(markers), w.CurrentZone),
		zenity.Title("Confirm Delete All"),
		zenity.OKLabel("Delete All"),
		zenity.CancelLabel("Cancel"),
	)
	w.dialogOpen = false
	w.lastMousePressed = true

	if err != nil {
		// User cancelled
		return
	}

	// Delete all markers in current zone
	delete(w.Config.Markers, w.CurrentZone)

	// Save to disk
	if err := w.Config.Save(); err != nil {
		fmt.Printf("❌ Error deleting markers: %v\n", err)
	} else {
		fmt.Printf("🗑️  Deleted all %d markers from %s\n", len(markers), w.CurrentZone)
	}
}

func (w *Window) editMarkerAt(worldX, worldY float64) {
	if w.CurrentZone == "" {
		return
	}

	markers, ok := w.Config.Markers[w.CurrentZone]
	if !ok || len(markers) == 0 {
		return
	}

	// Check if click is within range of any marker
	// Use a fixed click radius of 15 units in world space
	clickRadius := 15.0 / w.Zoom

	for i, marker := range markers {
		dx := worldX - marker.X
		dy := worldY - marker.Y
		distance := math.Sqrt(dx*dx + dy*dy)

		if distance <= clickRadius {
			// Show text input dialog for label
			w.dialogOpen = true
			newLabel, err := zenity.Entry(
				"Edit marker label:",
				zenity.Title("Edit Marker"),
				zenity.EntryText(marker.Label),
			)
			w.dialogOpen = false
			w.lastMousePressed = true // Prevent re-triggering on dialog close

			// If user cancelled, do nothing
			if err != nil {
				return
			}

			// If empty, keep existing label
			if newLabel == "" {
				newLabel = marker.Label
			}

			// Update the marker label
			w.Config.Markers[w.CurrentZone][i].Label = newLabel

			// Save to disk
			if err := w.Config.Save(); err != nil {
				fmt.Printf("❌ Error updating marker: %v\n", err)
			} else {
				fmt.Printf("📝 Marker updated: '%s' -> '%s' in %s\n", marker.Label, newLabel, w.CurrentZone)
			}

			return
		}
	}
}

func (w *Window) refitZoom() {
	if w.MapData == nil {
		return
	}

	data := w.MapData
	var minX, maxX, minY, maxY float64

	if w.ZLevelMode > 0 && w.LogReader != nil {
		// Calculate bounds for current Z-level
		var activeZ float64
		if w.ZLevelMode == 1 {
			activeZ = w.LogReader.CurrentState.Z
		} else {
			activeZ = w.ZLevelManual
		}

		minX, maxX = 99999.0, -99999.0
		minY, maxY = 99999.0, -99999.0
		foundVisibleLines := false

		for _, line := range data.Lines {
			z1InRange := math.Abs(line.Z1-activeZ) <= w.ZLevelRange
			z2InRange := math.Abs(line.Z2-activeZ) <= w.ZLevelRange
			if z1InRange || z2InRange {
				if line.X1 < minX { minX = line.X1 }
				if line.X1 > maxX { maxX = line.X1 }
				if line.Y1 < minY { minY = line.Y1 }
				if line.Y1 > maxY { maxY = line.Y1 }
				if line.X2 < minX { minX = line.X2 }
				if line.X2 > maxX { maxX = line.X2 }
				if line.Y2 < minY { minY = line.Y2 }
				if line.Y2 > maxY { maxY = line.Y2 }
				foundVisibleLines = true
			}
		}

		// If no visible lines, fall back to full map bounds
		if !foundVisibleLines {
			minX, maxX = data.MinX, data.MaxX
			minY, maxY = data.MinY, data.MaxY
		}
	} else {
		// Use full map bounds when Z-filtering is off
		minX, maxX = data.MinX, data.MaxX
		minY, maxY = data.MinY, data.MaxY
	}

	w.CamX = (minX + maxX) / 2
	w.CamY = (minY + maxY) / 2

	// Calculate zoom to fit visible geometry in window with some padding
	mapWidth := maxX - minX
	mapHeight := maxY - minY

	// Add 10% padding so map doesn't touch edges
	zoomX := float64(w.Width) * 0.9 / mapWidth
	zoomY := float64(w.Height) * 0.9 / mapHeight

	// Use the smaller zoom to ensure entire map fits
	if zoomX < zoomY {
		w.Zoom = zoomX
	} else {
		w.Zoom = zoomY
	}
}

func (w *Window) Draw(screen *ebiten.Image) {
	// Create offscreen image for all map content
	offscreen := ebiten.NewImage(w.Width, w.Height)
	offscreen.Fill(color.Black)

	cx, cy := float64(w.Width)/2, float64(w.Height)/2

	if w.MapData != nil {
		// Determine active Z level for filtering (if enabled)
		var activeZ float64
		if w.ZLevelMode == 1 && w.LogReader != nil {
			// Auto mode
			activeZ = w.LogReader.CurrentState.Z
		} else if w.ZLevelMode == 2 {
			// Manual mode
			activeZ = w.ZLevelManual
		}

		// DRAW LINES with stroke width for better visibility
		lineWidth := float32(1.5)
		if w.Zoom > 2.0 {
			lineWidth = float32(2.0)
		}

		for _, line := range w.MapData.Lines {
			// Z-Level filtering: skip lines outside the Z range (if mode is not off)
			if w.ZLevelMode > 0 {
				// Check if either endpoint is within range
				z1InRange := math.Abs(line.Z1-activeZ) <= w.ZLevelRange
				z2InRange := math.Abs(line.Z2-activeZ) <= w.ZLevelRange
				if !z1InRange && !z2InRange {
					continue
				}
			}

			x1 := float32((line.X1 - w.CamX) * w.Zoom + cx)
			y1 := float32((line.Y1 - w.CamY) * w.Zoom + cy)
			x2 := float32((line.X2 - w.CamX) * w.Zoom + cx)
			y2 := float32((line.Y2 - w.CamY) * w.Zoom + cy)
			vector.StrokeLine(offscreen, x1, y1, x2, y2, lineWidth, line.Color, true)
		}

		// DRAW LABELS (based on mode)
		// 0 = all, 1 = custom+zone lines, 2 = zone lines only, 3 = none
		if w.LabelMode < 3 {
			for _, lbl := range w.MapData.Labels {
				// Zone lines start with "to " (underscores were replaced with spaces)
				isZoneLine := len(lbl.Text) >= 3 && lbl.Text[:3] == "to "

				// Filter based on mode
				if w.LabelMode == 2 && !isZoneLine {
					// Mode 2: zone lines only - skip non-zone labels
					continue
				} else if w.LabelMode == 1 && !isZoneLine {
					// Mode 1: custom+zone lines - skip map labels (but custom markers will be drawn later)
					continue
				}

				lx := (lbl.X - w.CamX) * w.Zoom + cx
				ly := (lbl.Y - w.CamY) * w.Zoom + cy

				if lx > -50 && lx < float64(w.Width)+50 && ly > -50 && ly < float64(w.Height)+50 {
					text.Draw(offscreen, lbl.Text, basicfont.Face7x13, int(lx), int(ly), lbl.Color)
				}
			}
		}

		// DRAW BREADCRUMBS as filled circles (if enabled)
		if w.ShowBreadcrumbs {
			breadcrumbColor := color.RGBA{255, 255, 0, 200}
			breadcrumbSize := float32(1.5)
			for _, bc := range w.Breadcrumbs {
				bx := float32((bc.X - w.CamX) * w.Zoom + cx)
				by := float32((bc.Y - w.CamY) * w.Zoom + cy)
				vector.DrawFilledCircle(offscreen, bx, by, breadcrumbSize, breadcrumbColor, true)
			}
		}
	}

	// DRAW CUSTOM MARKERS for current zone
	if w.ShowMarkers {
		if markers, ok := w.Config.Markers[w.CurrentZone]; ok {
			for _, marker := range markers {
				mx := float32((marker.X - w.CamX) * w.Zoom + cx)
				my := float32((marker.Y - w.CamY) * w.Zoom + cy)

				// Get marker color
				markerColor := w.getMarkerColor(marker.Color)

				// Draw marker with selected shape
				w.drawMarkerShape(offscreen, mx, my, marker.Shape, markerColor)

				// Draw label based on label mode
				// 0 = all labels, 1 = custom+zone lines, 2 = zone lines only, 3 = none
				if w.LabelMode <= 1 {
					text.Draw(offscreen, marker.Label, basicfont.Face7x13, int(mx)+10, int(my)+4, color.RGBA{255, 200, 0, 255})
				}
			}
		}
	}

	// DRAW CORPSE MARKER (only if in same zone)
	if w.LogReader != nil && w.LogReader.CurrentState.HasCorpse && w.LogReader.CurrentState.CorpseZone == w.CurrentZone {
		w.drawCorpseMarker(offscreen, cx, cy)
	}

	// DRAW PLAYER ARROW
	if w.LogReader != nil {
		w.drawPlayerArrow(offscreen, cx, cy)
	}

	// Apply opacity to entire screen and enable filtering for anti-aliasing
	opts := &ebiten.DrawImageOptions{}
	opts.ColorScale.ScaleAlpha(float32(w.Opacity))
	opts.Filter = ebiten.FilterLinear
	screen.DrawImage(offscreen, opts)

	// DRAW UI / DEBUG (drawn after offscreen is composited, so UI is always at full opacity)
	w.drawUI(screen)
}

func (w *Window) drawCorpseMarker(screen *ebiten.Image, cx, cy float64) {
	s := w.LogReader.CurrentState

	// Convert Corpse World Pos to Screen Pos
	corpseX := float32((s.CorpseX - w.CamX) * w.Zoom + cx)
	corpseY := float32((s.CorpseY - w.CamY) * w.Zoom + cy)

	size := float32(12.0 * w.Zoom)
	if size < 10 { size = 10 }
	if size > 30 { size = 30 }

	c := color.RGBA{255, 0, 0, 255}

	// Draw filled circle background
	vector.DrawFilledCircle(screen, corpseX, corpseY, size, color.RGBA{255, 0, 0, 100}, true)

	// Draw stroke circle
	vector.StrokeCircle(screen, corpseX, corpseY, size, 2.5, c, true)

	// Draw X with thicker lines
	strokeWidth := float32(3.0)
	vector.StrokeLine(screen, corpseX-size*0.6, corpseY-size*0.6, corpseX+size*0.6, corpseY+size*0.6, strokeWidth, c, true)
	vector.StrokeLine(screen, corpseX-size*0.6, corpseY+size*0.6, corpseX+size*0.6, corpseY-size*0.6, strokeWidth, c, true)
}

func (w *Window) drawPlayerArrow(screen *ebiten.Image, cx, cy float64) {
	s := w.LogReader.CurrentState

	// Convert Player World Pos to Screen Pos
	px := float32((s.X - w.CamX) * w.Zoom + cx)
	py := float32((s.Y - w.CamY) * w.Zoom + cy)

	// Heading
	angle := s.Heading

	size := float32(10.0 * w.Zoom)
	if size < 8 { size = 8 }
	if size > 25 { size = 25 }

	// Calculate arrow points
	x1 := px + float32(math.Cos(angle))*size
	y1 := py + float32(math.Sin(angle))*size

	x2 := px + float32(math.Cos(angle + 2.6))*size
	y2 := py + float32(math.Sin(angle + 2.6))*size

	x3 := px + float32(math.Cos(angle - 2.6))*size
	y3 := py + float32(math.Sin(angle - 2.6))*size

	c := color.RGBA{0, 255, 0, 255}

	// Draw filled triangle for better visibility
	var path vector.Path
	path.MoveTo(x1, y1)
	path.LineTo(x2, y2)
	path.LineTo(x3, y3)
	path.Close()

	// Fill the arrow
	vertices, indices := path.AppendVerticesAndIndicesForFilling(nil, nil)
	for i := range vertices {
		vertices[i].ColorR = float32(c.R) / 255.0
		vertices[i].ColorG = float32(c.G) / 255.0
		vertices[i].ColorB = float32(c.B) / 255.0
		vertices[i].ColorA = float32(c.A) / 255.0
	}
	screen.DrawTriangles(vertices, indices, ebiten.NewImage(1, 1).SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image), &ebiten.DrawTrianglesOptions{
		AntiAlias: true,
	})

	// Draw stroke outline for better definition
	strokeWidth := float32(1.5)
	vector.StrokeLine(screen, x1, y1, x2, y2, strokeWidth, c, true)
	vector.StrokeLine(screen, x2, y2, x3, y3, strokeWidth, c, true)
	vector.StrokeLine(screen, x3, y3, x1, y1, strokeWidth, c, true)
}

type MenuButton struct {
	X, Y, W, H int
	Label      string
	Action     func()
	GetState   func() string
}

type MenuItem struct {
	Label   string
	Hotkey  string     // Optional hotkey text (e.g., "L", "Space", "PgUp")
	Action  func()
	Submenu []MenuItem // For nested menus
}

type Menu struct {
	Label string
	Items []MenuItem
}

// calculateMenuWidth calculates the width of a dropdown menu based on its items
func calculateMenuWidth(items []MenuItem) int {
	maxLabelWidth := 0
	maxHotkeyWidth := 0
	for _, item := range items {
		labelWidth := len(item.Label) * 7
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
		if item.Hotkey != "" {
			hotkeyWidth := len(item.Hotkey) * 7
			if hotkeyWidth > maxHotkeyWidth {
				maxHotkeyWidth = hotkeyWidth
			}
		}
	}
	// Total width: left padding + label + gap + hotkey + right padding
	maxWidth := 16 + maxLabelWidth + 16 + maxHotkeyWidth + 16
	if maxWidth < 150 {
		maxWidth = 150
	}
	return maxWidth
}

func (w *Window) drawUI(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	cx, cy := float64(w.Width)/2, float64(w.Height)/2

	// Reverse transform: Screen -> World (map coordinates)
	worldX := (float64(mx) - cx) / w.Zoom + w.CamX
	worldY := (float64(my) - cy) / w.Zoom + w.CamY

	// Convert to EQ /loc format (Y, X with negation reversed)
	mouseLocY := -worldY
	mouseLocX := -worldX
	playerLocY := -w.LogReader.CurrentState.Y
	playerLocX := -w.LogReader.CurrentState.X

	// Define menus
	labelModes := []string{"ALL", "CUSTOM + ZONE LINES", "ZONE LINES", "NONE"}
	zModes := []string{"OFF", "AUTO", "MANUAL"}

	menus := []Menu{
		{
			Label: "File",
			Items: []MenuItem{
				{
					Label: "Set EQ Path...",
					Action: func() {
						dir, err := zenity.SelectFile(
							zenity.Title("Select EverQuest Directory"),
							zenity.Directory(),
						)
						if err == nil && dir != "" {
							w.Config.EQPath = dir
							if err := w.Config.Save(); err != nil {
								fmt.Printf("Error saving config: %v\n", err)
							} else {
								fmt.Printf("✅ EQ Path saved: %s\n", dir)
								fmt.Println("Please restart the application for changes to take effect.")
							}
						}
						w.openMenu = ""
					},
				},
				{
					Label: "Exit",
					Action: func() {
						os.Exit(0)
					},
				},
			},
		},
		{
			Label: "View",
			Items: []MenuItem{
				{
					Label: fmt.Sprintf("Info Panel: %s", map[bool]string{true: "ON", false: "OFF"}[w.showInfo]),
					Action: func() {
						w.showInfo = !w.showInfo
						w.openMenu = ""
					},
				},
				{
					Label: fmt.Sprintf("Labels: %s", labelModes[w.LabelMode]),
					Hotkey: "L",
					Action: func() {
						w.LabelMode = (w.LabelMode + 1) % 4
						w.openMenu = ""
					},
				},
				{
					Label: fmt.Sprintf("Breadcrumbs: %s", map[bool]string{true: "ON", false: "OFF"}[w.ShowBreadcrumbs]),
					Hotkey: "B",
					Action: func() {
						w.ShowBreadcrumbs = !w.ShowBreadcrumbs
						w.openMenu = ""
					},
				},
				{
					Label: fmt.Sprintf("Markers: %s", map[bool]string{true: "ON", false: "OFF"}[w.ShowMarkers]),
					Hotkey: "R",
					Action: func() {
						w.ShowMarkers = !w.ShowMarkers
						w.openMenu = ""
					},
				},
				{
					Label: fmt.Sprintf("Z-Level: %s", zModes[w.ZLevelMode]),
					Hotkey: "Z",
					Action: func() {
						w.ZLevelMode = (w.ZLevelMode + 1) % 3
						if w.ZLevelMode == 2 && w.LogReader != nil {
							w.ZLevelManual = w.LogReader.CurrentState.Z
						}
						w.openMenu = ""
					},
				},
				{
					Label: "Opacity +",
					Hotkey: "=",
					Action: func() {
						w.Opacity += 0.1
						if w.Opacity > 1.0 { w.Opacity = 1.0 }
						w.openMenu = ""
					},
				},
				{
					Label: "Opacity -",
					Hotkey: "-",
					Action: func() {
						w.Opacity -= 0.1
						if w.Opacity < 0.1 { w.Opacity = 0.1 }
						w.openMenu = ""
					},
				},
			},
		},
		{
			Label: "Tools",
			Items: []MenuItem{
				{
					Label: "Center on Player",
					Hotkey: "Space",
					Action: func() {
						if w.LogReader != nil {
							w.CamX = w.LogReader.CurrentState.X
							w.CamY = w.LogReader.CurrentState.Y
						}
						w.openMenu = ""
					},
				},
				{
					Label: "Fit Map to Window",
					Hotkey: "Home",
					Action: func() {
						w.refitZoom()
						w.openMenu = ""
					},
				},
				{
					Label: "Z-Level Up",
					Hotkey: "PgUp",
					Action: func() {
						w.ZLevelManual += 10.0
						w.ZLevelMode = 2
						w.openMenu = ""
					},
				},
				{
					Label: "Z-Level Down",
					Hotkey: "PgDn",
					Action: func() {
						w.ZLevelManual -= 10.0
						w.ZLevelMode = 2
						w.openMenu = ""
					},
				},
				{
					Label: "Z-Range Increase",
					Hotkey: "Ins",
					Action: func() {
						w.ZLevelRange += 10.0
						if w.ZLevelRange > 200.0 { w.ZLevelRange = 200.0 }
						w.openMenu = ""
					},
				},
				{
					Label: "Z-Range Decrease",
					Hotkey: "Del",
					Action: func() {
						w.ZLevelRange -= 10.0
						if w.ZLevelRange < 10.0 { w.ZLevelRange = 10.0 }
						w.openMenu = ""
					},
				},
			},
		},
		{
			Label: "Markers",
			Items: []MenuItem{
				{
					Label: fmt.Sprintf("Place Marker: %s", map[bool]string{true: "ON", false: "OFF"}[w.placingMarker]),
					Hotkey: "M",
					Action: func() {
						w.placingMarker = !w.placingMarker
						w.openMenu = ""
					},
				},
				{
					Label: fmt.Sprintf("Color: %s", w.markerColor),
					Submenu: []MenuItem{
						{
							Label: "Red",
							Action: func() {
								w.markerColor = "red"
								w.openMenu = ""
							},
						},
						{
							Label: "Blue",
							Action: func() {
								w.markerColor = "blue"
								w.openMenu = ""
							},
						},
						{
							Label: "Green",
							Action: func() {
								w.markerColor = "green"
								w.openMenu = ""
							},
						},
						{
							Label: "Yellow",
							Action: func() {
								w.markerColor = "yellow"
								w.openMenu = ""
							},
						},
						{
							Label: "Purple",
							Action: func() {
								w.markerColor = "purple"
								w.openMenu = ""
							},
						},
					},
				},
				{
					Label: fmt.Sprintf("Shape: %s", w.markerShape),
					Submenu: []MenuItem{
						{
							Label: "Circle",
							Action: func() {
								w.markerShape = "circle"
								w.openMenu = ""
							},
						},
						{
							Label: "Square",
							Action: func() {
								w.markerShape = "square"
								w.openMenu = ""
							},
						},
						{
							Label: "Triangle",
							Action: func() {
								w.markerShape = "triangle"
								w.openMenu = ""
							},
						},
						{
							Label: "Diamond",
							Action: func() {
								w.markerShape = "diamond"
								w.openMenu = ""
							},
						},
						{
							Label: "Star",
							Action: func() {
								w.markerShape = "star"
								w.openMenu = ""
							},
						},
					},
				},
			},
		},
	}

	// Add conditional menu items
	if w.ShowBreadcrumbs && len(w.Breadcrumbs) > 0 {
		menus[2].Items = append(menus[2].Items, MenuItem{ // Tools menu
			Label: "Clear Breadcrumbs",
			Hotkey: "C",
			Action: func() {
				w.Breadcrumbs = w.Breadcrumbs[:0]
				w.openMenu = ""
			},
		})
	}

	if w.LogReader != nil && w.LogReader.CurrentState.HasCorpse {
		menus[2].Items = append(menus[2].Items, MenuItem{ // Tools menu
			Label: "Clear Corpse Marker",
			Hotkey: "K",
			Action: func() {
				w.LogReader.CurrentState.HasCorpse = false
				w.openMenu = ""
			},
		})
	}

	// Add conditional marker menu items
	if w.CurrentZone != "" {
		if markers, ok := w.Config.Markers[w.CurrentZone]; ok && len(markers) > 0 {
			menus[3].Items = append(menus[3].Items, MenuItem{ // Markers menu
				Label: fmt.Sprintf("Clear All (%d markers)", len(markers)),
				Action: func() {
					w.openMenu = ""
					w.clearAllMarkers()
				},
			})
		}
	}

	// Handle submenu hover (before click handling)
	if w.openMenu != "" {
		x := 0
		newSubmenu := -1 // Track what submenu should be open
		for _, menu := range menus {
			menuWidth := len(menu.Label)*7 + 16
			if menu.Label == w.openMenu {
				maxWidth := calculateMenuWidth(menu.Items)

				// First pass: Check if hovering directly over a menu item with submenu
				dropY := w.menuBarHeight
				for i, item := range menu.Items {
					itemY := dropY + i*20

					// Check if hovering over the menu item itself
					if mx >= x && mx < x+maxWidth && my >= itemY && my < itemY+20 {
						if len(item.Submenu) > 0 {
							newSubmenu = i
						}
						break
					}
				}

				// Second pass: If not hovering over a menu item, check if mouse is in the CURRENTLY OPEN submenu area
				if newSubmenu == -1 && w.openSubmenu >= 0 && w.openSubmenu < len(menu.Items) {
					item := menu.Items[w.openSubmenu]
					if len(item.Submenu) > 0 {
						itemY := dropY + w.openSubmenu*20
						submenuX := x + maxWidth
						submenuY := itemY
						submenuHeight := len(item.Submenu) * 20
						if mx >= submenuX && mx < submenuX+150 && my >= submenuY && my < submenuY+submenuHeight {
							newSubmenu = w.openSubmenu
						}
					}
				}
				break
			}
			x += menuWidth
		}
		w.openSubmenu = newSubmenu
	}

	// Handle menu interactions
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !w.lastMousePressed {
			handled := false

			// Check menu bar clicks
			if my < w.menuBarHeight {
				x := 0
				for _, menu := range menus {
					menuWidth := len(menu.Label)*7 + 16
					if mx >= x && mx < x+menuWidth {
						if w.openMenu == menu.Label {
							w.openMenu = ""
							w.openSubmenu = -1
						} else {
							w.openMenu = menu.Label
							w.openSubmenu = -1
						}
						handled = true
						break
					}
					x += menuWidth
				}
			}

			// Check submenu clicks first
			if !handled && w.openMenu != "" && w.openSubmenu >= 0 {
				x := 0
				for _, menu := range menus {
					menuWidth := len(menu.Label)*7 + 16
					if menu.Label == w.openMenu {
						maxWidth := calculateMenuWidth(menu.Items)

						if w.openSubmenu < len(menu.Items) {
							submenu := menu.Items[w.openSubmenu].Submenu
							dropY := w.menuBarHeight
							submenuX := x + maxWidth
							submenuY := dropY + w.openSubmenu*20

							for _, subitem := range submenu {
								if mx >= submenuX && mx < submenuX+150 && my >= submenuY && my < submenuY+20 {
									subitem.Action()
									handled = true
									break
								}
								submenuY += 20
							}
						}
						break
					}
					x += menuWidth
				}
			}

			// Check dropdown clicks
			if !handled && w.openMenu != "" {
				x := 0
				for _, menu := range menus {
					menuWidth := len(menu.Label)*7 + 16
					if menu.Label == w.openMenu {
						maxWidth := calculateMenuWidth(menu.Items)

						// Check if click is in dropdown
						dropY := w.menuBarHeight
						for i, item := range menu.Items {
							itemY := dropY + i*20
							if mx >= x && mx < x+maxWidth && my >= itemY && my < itemY+20 {
								// Only execute if no submenu
								if len(item.Submenu) == 0 && item.Action != nil {
									item.Action()
									handled = true
								}
								break
							}
						}
						break
					}
					x += menuWidth
				}
			}

			// Close menu if clicked outside
			if !handled && w.openMenu != "" {
				w.openMenu = ""
				w.openSubmenu = -1
			}
		}
		w.lastMousePressed = true
	} else {
		w.lastMousePressed = false
	}

	// Draw menu bar
	menuBar := ebiten.NewImage(w.Width, w.menuBarHeight)
	menuBar.Fill(color.RGBA{240, 240, 240, 255})
	screen.DrawImage(menuBar, nil)

	// Draw menu labels
	x := 0
	for _, menu := range menus {
		menuWidth := len(menu.Label)*7 + 16

		// Highlight if hovered or open
		if (mx >= x && mx < x+menuWidth && my < w.menuBarHeight) || w.openMenu == menu.Label {
			highlight := ebiten.NewImage(menuWidth, w.menuBarHeight)
			highlight.Fill(color.RGBA{200, 200, 200, 255})
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), 0)
			screen.DrawImage(highlight, op)
		}

		text.Draw(screen, menu.Label, basicfont.Face7x13, x+8, 16, color.Black)
		x += menuWidth
	}

	// Draw info text below menu bar (if enabled)
	if w.showInfo {
		infoY := w.menuBarHeight + 8

		// Status info only
		statusInfo := []string{
			fmt.Sprintf("Zone: %s", w.CurrentZone),
			fmt.Sprintf("Player: %.1f, %.1f", playerLocY, playerLocX),
			fmt.Sprintf("Mouse: %.1f, %.1f", mouseLocY, mouseLocX),
		}

		if w.MapData != nil {
			statusInfo = append(statusInfo, fmt.Sprintf("Map: X[%.0f to %.0f] Y[%.0f to %.0f]",
				w.MapData.MinX, w.MapData.MaxX, w.MapData.MinY, w.MapData.MaxY))
		}

		// Z-Level info
		zModeLabels := []string{"OFF", "AUTO", "MANUAL"}
		if w.ZLevelMode == 1 && w.LogReader != nil {
			statusInfo = append(statusInfo, fmt.Sprintf("Z-Level: %.1f ±%.0f (%s)", w.LogReader.CurrentState.Z, w.ZLevelRange, zModeLabels[w.ZLevelMode]))
		} else if w.ZLevelMode == 2 {
			statusInfo = append(statusInfo, fmt.Sprintf("Z-Level: %.1f ±%.0f (%s)", w.ZLevelManual, w.ZLevelRange, zModeLabels[w.ZLevelMode]))
		} else {
			statusInfo = append(statusInfo, fmt.Sprintf("Z-Level: %s", zModeLabels[w.ZLevelMode]))
		}

		statusInfo = append(statusInfo, fmt.Sprintf("Zoom: %.2fx | Opacity: %.0f%%", w.Zoom, w.Opacity*100))

		// Marker placement mode indicator
		if w.placingMarker {
			statusInfo = append(statusInfo, fmt.Sprintf(">>> PLACING MARKER (%s %s) <<<", w.markerColor, w.markerShape))
		}

		ebitenutil.DebugPrintAt(screen, strings.Join(statusInfo, "\n"), 8, infoY)
	}

	// Draw crosshair when in marker placement mode
	if w.placingMarker && my > w.menuBarHeight {
		markerColor := w.getMarkerColor(w.markerColor)
		// Draw crosshair at mouse position
		crosshairSize := float32(20)
		vector.StrokeLine(screen, float32(mx)-crosshairSize, float32(my), float32(mx)+crosshairSize, float32(my), 2, markerColor, true)
		vector.StrokeLine(screen, float32(mx), float32(my)-crosshairSize, float32(mx), float32(my)+crosshairSize, 2, markerColor, true)
		// Draw preview of marker shape at cursor
		w.drawMarkerShape(screen, float32(mx), float32(my), w.markerShape, color.RGBA{
			R: markerColor.R,
			G: markerColor.G,
			B: markerColor.B,
			A: 128, // Semi-transparent preview
		})
	}

	// Draw dropdown menu if open (drawn last so it appears on top)
	if w.openMenu != "" {
		x := 0
		for _, menu := range menus {
			menuWidth := len(menu.Label)*7 + 16
			if menu.Label == w.openMenu {
				maxWidth := calculateMenuWidth(menu.Items)

				// Draw dropdown background
				dropHeight := len(menu.Items) * 20
				dropdown := ebiten.NewImage(maxWidth, dropHeight)
				dropdown.Fill(color.RGBA{250, 250, 250, 255})

				// Draw border
				vector.StrokeRect(screen, float32(x), float32(w.menuBarHeight), float32(maxWidth), float32(dropHeight), 1, color.RGBA{180, 180, 180, 255}, false)

				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(x), float64(w.menuBarHeight))
				screen.DrawImage(dropdown, op)

				// Draw items
				for i, item := range menu.Items {
					itemY := w.menuBarHeight + i*20

					// Highlight if hovered or has submenu open
					if (mx >= x && mx < x+maxWidth && my >= itemY && my < itemY+20) || w.openSubmenu == i {
						itemBg := ebiten.NewImage(maxWidth, 20)
						itemBg.Fill(color.RGBA{200, 200, 255, 255})
						itemOp := &ebiten.DrawImageOptions{}
						itemOp.GeoM.Translate(float64(x), float64(itemY))
						screen.DrawImage(itemBg, itemOp)
					}

					// Draw label on left
					text.Draw(screen, item.Label, basicfont.Face7x13, x+8, itemY+14, color.Black)

					// Draw submenu indicator (triangle) if item has submenu
					if len(item.Submenu) > 0 {
						// Draw a simple ">" character to indicate submenu
						triX := x + maxWidth - 12
						triY := itemY + 14
						text.Draw(screen, ">", basicfont.Face7x13, triX, triY, color.Black)
					}

					// Draw hotkey on right (if it exists)
					if item.Hotkey != "" {
						hotkeyX := x + maxWidth - len(item.Hotkey)*7 - 8
						text.Draw(screen, item.Hotkey, basicfont.Face7x13, hotkeyX, itemY+14, color.Black)
					}
				}

				// Draw submenu if open
				if w.openSubmenu >= 0 && w.openSubmenu < len(menu.Items) {
					submenu := menu.Items[w.openSubmenu].Submenu
					if len(submenu) > 0 {
						submenuX := x + maxWidth
						submenuY := w.menuBarHeight + w.openSubmenu*20
						submenuHeight := len(submenu) * 20

						// Draw submenu background
						submenuBg := ebiten.NewImage(150, submenuHeight)
						submenuBg.Fill(color.RGBA{250, 250, 250, 255})

						// Draw border
						vector.StrokeRect(screen, float32(submenuX), float32(submenuY), 150, float32(submenuHeight), 1, color.RGBA{180, 180, 180, 255}, false)

						subOp := &ebiten.DrawImageOptions{}
						subOp.GeoM.Translate(float64(submenuX), float64(submenuY))
						screen.DrawImage(submenuBg, subOp)

						// Draw submenu items
						for j, subitem := range submenu {
							subitemY := submenuY + j*20

							// Highlight if hovered
							if mx >= submenuX && mx < submenuX+150 && my >= subitemY && my < subitemY+20 {
								subitemBg := ebiten.NewImage(150, 20)
								subitemBg.Fill(color.RGBA{200, 200, 255, 255})
								subitemOp := &ebiten.DrawImageOptions{}
								subitemOp.GeoM.Translate(float64(submenuX), float64(subitemY))
								screen.DrawImage(subitemBg, subitemOp)
							}

							text.Draw(screen, subitem.Label, basicfont.Face7x13, submenuX+8, subitemY+14, color.Black)
						}
					}
				}

				break
			}
			x += menuWidth
		}
	}
}

func (w *Window) Layout(outsideWidth, outsideHeight int) (int, int) {
	w.Width = outsideWidth
	w.Height = outsideHeight
	return outsideWidth, outsideHeight
}