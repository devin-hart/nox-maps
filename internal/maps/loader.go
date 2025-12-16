package maps

import (
	"bufio"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type MapLine struct {
	X1, Y1, Z1 float64
	X2, Y2, Z2 float64
	Color      color.RGBA
}

type MapLabel struct {
	X, Y, Z float64
	Color   color.RGBA
	Size    int
	Text    string
}

type ZoneMap struct {
	Name   string
	Lines  []MapLine
	Labels []MapLabel
	MinX, MaxX float64
	MinY, MaxY float64
}

func LoadZone(mapDir, zoneName string) (*ZoneMap, error) {
	zm := &ZoneMap{
		Name: zoneName,
		MinX: 99999, MaxX: -99999,
		MinY: 99999, MaxY: -99999,
	}

	// 1. Build a case-insensitive map of all files in the directory
	// This ensures we find "EastKarana.txt" even if we ask for "eastkarana.txt"
	globPattern := filepath.Join(mapDir, "*")
	allFiles, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, fmt.Errorf("could not list map directory: %v", err)
	}

	fileMap := make(map[string]string)
	for _, path := range allFiles {
		filename := filepath.Base(path)
		lower := strings.ToLower(filename)
		fileMap[lower] = path
	}

	// 2. Identify target files (Base + Layers 1-3)
	targets := []string{
		strings.ToLower(fmt.Sprintf("%s.txt", zoneName)),
		strings.ToLower(fmt.Sprintf("%s_1.txt", zoneName)),
		strings.ToLower(fmt.Sprintf("%s_2.txt", zoneName)),
		strings.ToLower(fmt.Sprintf("%s_3.txt", zoneName)),
	}

	// 3. Load them
	foundAtLeastOne := false
	for _, target := range targets {
		if realPath, exists := fileMap[target]; exists {
			fmt.Printf("📄 Parsing: %s ... ", filepath.Base(realPath))
			itemsAdded, err := zm.parseFile(realPath)
			if err == nil && itemsAdded > 0 {
				foundAtLeastOne = true
				fmt.Printf("OK (%d items)\n", itemsAdded)
			} else {
				// Don't panic, just report
				fmt.Printf("Found 0 valid items. (might be empty or bad format)\n")
			}
		}
	}

	if !foundAtLeastOne {
		// DEBUG: Print what we *did* see to help diagnose
		fmt.Printf("\n⚠️ Could not find maps for '%s'. \nAre they in '%s'?\nHere are 5 random files I see in that folder:\n", zoneName, mapDir)
		count := 0
		for _, f := range allFiles {
			if count < 5 {
				fmt.Printf(" - %s\n", filepath.Base(f))
				count++
			}
		}
		return nil, fmt.Errorf("no map files found for zone: %s", zoneName)
	}

	return zm, nil
}

func (zm *ZoneMap) parseFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	
	for scanner.Scan() {
		rawLine := scanner.Text()
		
		// 1. Sanitize
		line := strings.ReplaceAll(rawLine, "\ufeff", "") 
		line = strings.TrimSpace(line)
		if line == "" { continue }

		// 2. HUNT for the start of the command
		// Lines might start with "L ..."
		// We look for the first occurrence of 'L' or 'P' that is followed by a number or space/comma
		cmdIndex := -1
		cmdType := '?'

		for i, r := range line {
			if unicode.ToUpper(r) == 'L' || unicode.ToUpper(r) == 'P' {
				cmdIndex = i
				cmdType = unicode.ToUpper(r)
				break
			}
		}

		if cmdIndex == -1 { continue } // No command found

		// Extract the useful part: "123.45, 67.89, ..."
		// We skip the command char (L/P) and any leading junk
		content := line[cmdIndex+1:]
		content = strings.TrimLeft(content, " ,")
		parts := strings.Split(content, ",")

		if cmdType == 'L' {
			// EQ Map Format: eqY, eqX, z, eqY, eqX, z, r, g, b
			// Where eqY is S/N axis, eqX is E/W axis
			// Convert to screen coords: X=eqY (S/N becomes horizontal), Y=eqX (E/W becomes vertical)
			if len(parts) >= 6 {
				eqY1 := parseFloat(parts[0])
				eqX1 := parseFloat(parts[1])
				eqY2 := parseFloat(parts[3])
				eqX2 := parseFloat(parts[4])

				l := MapLine{
					X1: eqY1, Y1: eqX1, Z1: parseFloat(parts[2]),
					X2: eqY2, Y2: eqX2, Z2: parseFloat(parts[5]),
				}
				if len(parts) >= 9 {
					l.Color = parseColor(parts[6], parts[7], parts[8])
				} else {
					l.Color = color.RGBA{150, 150, 150, 255}
				}
				zm.Lines = append(zm.Lines, l)
				zm.updateBounds(l.X1, l.Y1)
				zm.updateBounds(l.X2, l.Y2)
				count++
			}
		} else if cmdType == 'P' {
			// EQ Map Format: eqY, eqX, z, r, g, b, size, text...
			// Where eqY is S/N axis, eqX is E/W axis
			// Convert to screen coords: X=eqY, Y=eqX
			if len(parts) >= 7 {
				eqY := parseFloat(parts[0])
				eqX := parseFloat(parts[1])

				p := MapLabel{
					X: eqY, Y: eqX, Z: parseFloat(parts[2]),
					Color: parseColor(parts[3], parts[4], parts[5]),
					Size:  parseInt(parts[6]),
				}
				if len(parts) >= 8 {
					p.Text = strings.TrimSpace(strings.Join(parts[7:], ","))
					// Clean up underscores often used in EQ maps
					p.Text = strings.ReplaceAll(p.Text, "_", " ")
				}
				zm.Labels = append(zm.Labels, p)
				count++
			}
		}
	}
	return count, nil
}

func (zm *ZoneMap) updateBounds(x, y float64) {
	if x < zm.MinX { zm.MinX = x }
	if x > zm.MaxX { zm.MaxX = x }
	if y < zm.MinY { zm.MinY = y }
	if y > zm.MaxY { zm.MaxY = y }
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(s))
	return i
}

func parseColor(r, g, b string) color.RGBA {
	ri := parseInt(r)
	gi := parseInt(g)
	bi := parseInt(b)
	if ri == 0 && gi == 0 && bi == 0 {
		return color.RGBA{130, 130, 130, 255}
	}
	return color.RGBA{uint8(ri), uint8(gi), uint8(bi), 255}
}