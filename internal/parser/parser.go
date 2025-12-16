package parser

import (
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

func (e *Engine) ProcessLines(lines <-chan eqlog.LogLine) {
	locRegex := regexp.MustCompile(`Your Location is ([0-9.-]+), ([0-9.-]+), ([0-9.-]+)`)
	zoneRegex := regexp.MustCompile(`You have entered (.+)\.`)

	// Track previous position to calculate heading
	var lastX, lastY float64
	var hasMoved bool

	for logEntry := range lines {
		line := logEntry.Line

		// 1. POSITION & HEADING
		if matches := locRegex.FindStringSubmatch(line); len(matches) == 4 {
			y, _ := strconv.ParseFloat(matches[1], 64)
			x, _ := strconv.ParseFloat(matches[2], 64)
			
			// Calculate Heading based on movement vector
			if hasMoved {
				dx := x - lastX
				dy := y - lastY

				// Only update heading if we moved a noticeable amount (ignore jitters)
				if math.Abs(dx) > 0.1 || math.Abs(dy) > 0.1 {
					// Atan2(y, x) gives us the angle in radians from the X axis
					e.CurrentState.Heading = math.Atan2(dy, dx)
				}
			}

			e.CurrentState.Y = y
			e.CurrentState.X = x
			
			lastX = x
			lastY = y
			hasMoved = true
		}

		// 2. ZONE
		if matches := zoneRegex.FindStringSubmatch(line); len(matches) == 2 {
			e.CurrentState.Zone = matches[1]
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