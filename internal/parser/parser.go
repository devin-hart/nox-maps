package parser

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/devin-hart/nox-maps/internal/eqlog"
)

type PlayerState struct {
	X, Y, Z    float64
	Heading    float64
	Zone       string
	
	// CORPSE STATE
	CorpseX    float64
	CorpseY    float64
	HasCorpse  bool
}

type Engine struct {
	CurrentState PlayerState
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) ProcessLines(reader *eqlog.Reader, lines <-chan eqlog.LogLine) {
	// Compile regexes once
	locRegex := regexp.MustCompile(`Your Location is ([0-9.-]+), ([0-9.-]+), ([0-9.-]+)`)
	zoneRegex := regexp.MustCompile(`You have entered (.+)\.`)

	// Set initial zone if detected from log history
	if reader.InitialZone != "" {
		e.CurrentState.Zone = reader.InitialZone
		fmt.Printf("🗺️  Starting with zone: '%s'\n", reader.InitialZone)
	}

	// Track previous position to calculate heading
	var lastX, lastY float64
	var hasMoved bool

	for logEntry := range lines {
		line := logEntry.Line

		// 1. POSITION & HEADING
		if matches := locRegex.FindStringSubmatch(line); len(matches) == 4 {
			eqY, _ := strconv.ParseFloat(matches[1], 64)
			eqX, _ := strconv.ParseFloat(matches[2], 64)
			eqZ, _ := strconv.ParseFloat(matches[3], 64)

			// Map files use SWAPPED and NEGATED coordinates compared to /loc output
			x := -eqX
			y := -eqY

			if !hasMoved {
				fmt.Printf("📍 First position - EQ: (%.1f, %.1f) -> Map: (%.1f, %.1f)\n", eqY, eqX, x, y)
				hasMoved = true
			} else {
				// Calculate heading based on movement
				dx := x - lastX
				dy := y - lastY
				if math.Abs(dx) > 0.1 || math.Abs(dy) > 0.1 {
					e.CurrentState.Heading = math.Atan2(dy, dx)
				}
			}

			e.CurrentState.X = x
			e.CurrentState.Y = y
			e.CurrentState.Z = eqZ
			lastX = x
			lastY = y
			continue
		}

		// 2. ZONE
		if matches := zoneRegex.FindStringSubmatch(line); len(matches) == 2 {
			newZone := matches[1]
			if newZone != e.CurrentState.Zone {
				fmt.Printf("🌍 Zone detected: '%s'\n", newZone)
				e.CurrentState.Zone = newZone
			}
			continue
		}

		// 3. DEATH
		if strings.Contains(line, "You have been slain") {
			e.CurrentState.CorpseX = e.CurrentState.X
			e.CurrentState.CorpseY = e.CurrentState.Y
			e.CurrentState.HasCorpse = true
			continue
		}

		// 4. RECOVERY
		if strings.Contains(line, "You summon your corpse") {
			e.CurrentState.HasCorpse = false
		}
	}
}