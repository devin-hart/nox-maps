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
			// EQ Log Format: "Your Location is eqY, eqX, Z"
			// Where eqY is S/N axis, eqX is E/W axis
			eqY, _ := strconv.ParseFloat(matches[1], 64)
			eqX, _ := strconv.ParseFloat(matches[2], 64)

			// Map files use SWAPPED and NEGATED coordinates compared to /loc output
			// /loc shows: eqY, eqX  ->  Map stores: -eqX, -eqY
			// So to convert /loc to map coords: swap then negate
			x := -eqX
			y := -eqY

			// DEBUG: Print raw coords on first position
			if !hasMoved {
				fmt.Printf("📍 First position - EQ: (%.1f, %.1f) -> Map: (%.1f, %.1f)\n", eqY, eqX, x, y)
			}

			// Calculate Heading based on movement vector
			if hasMoved {
				dx := x - lastX
				dy := y - lastY

				// Only update heading if we moved a noticeable amount (ignore jitters)
				if math.Abs(dx) > 0.1 || math.Abs(dy) > 0.1 {
					// Atan2(dy, dx) gives us the angle in radians from the X axis
					e.CurrentState.Heading = math.Atan2(dy, dx)
				}
			}

			e.CurrentState.X = x
			e.CurrentState.Y = y

			lastX = x
			lastY = y
			hasMoved = true
		}

		// 2. ZONE
		if matches := zoneRegex.FindStringSubmatch(line); len(matches) == 2 {
			newZone := matches[1]
			if newZone != e.CurrentState.Zone {
				fmt.Printf("🌍 Zone detected: '%s'\n", newZone)
				e.CurrentState.Zone = newZone
			}
		}

		// 3. DEATH
		if strings.Contains(line, "You have been slain") {
			e.CurrentState.CorpseX = e.CurrentState.X
			e.CurrentState.CorpseY = e.CurrentState.Y
			e.CurrentState.HasCorpse = true
		}

		// 4. RECOVERY
		if strings.Contains(line, "You summon your corpse") {
			e.CurrentState.HasCorpse = false
		}
	}
}