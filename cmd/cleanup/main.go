package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// 1. Load the valid keys from map_keys.json
	validPrefixes, err := loadValidPrefixes("assets/maps/map_keys.json")
	if err != nil {
		panic(fmt.Sprintf("Failed to load keys: %v", err))
	}

	// 2. Walk the directory and cleanup
	dir := "assets/maps"
	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Scanning %s...\n", dir)
	deletedCount := 0
	keptCount := 0

	for _, file := range files {
		// Skip directories and the config file itself
		if file.IsDir() || file.Name() == "map_keys.json" || file.Name() == "map_keys.ini" {
			continue
		}

		// Only look at .txt files (standard EQ map format)
		if filepath.Ext(file.Name()) != ".txt" {
			continue
		}

		// Check if the file matches a valid zone
		if shouldKeepFile(file.Name(), validPrefixes) {
			keptCount++
		} else {
			// DELETE THE FILE
			fullPath := filepath.Join(dir, file.Name())
			if err := os.Remove(fullPath); err != nil {
				fmt.Printf("Error deleting %s: %v\n", file.Name(), err)
			} else {
				fmt.Printf("Deleted: %s\n", file.Name())
				deletedCount++
			}
		}
	}

	fmt.Printf("\nDone. Kept %d files. Deleted %d files.\n", keptCount, deletedCount)
}

func loadValidPrefixes(path string) (map[string]bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mapping map[string]string
	if err := json.NewDecoder(file).Decode(&mapping); err != nil {
		return nil, err
	}

	prefixes := make(map[string]bool)
	for _, code := range mapping {
		// Store as lowercase to ensure case-insensitive matching later
		prefixes[strings.ToLower(code)] = true
	}
	return prefixes, nil
}

func shouldKeepFile(filename string, prefixes map[string]bool) bool {
	lowerName := strings.ToLower(filename)
	
	// Remove extension
	baseName := strings.TrimSuffix(lowerName, ".txt")

	// 1. Exact match (e.g. "oot.txt")
	if prefixes[baseName] {
		return true
	}

	// 2. Layer match (e.g. "oot_1.txt", "oot_2.txt")
	// Split by underscore to find the base prefix
	parts := strings.Split(baseName, "_")
	if len(parts) > 1 {
		// Reassemble the prefix if it contains underscores (rare but possible), 
		// or just take the first part. 
		// Standard EQ format is usually {code}_{layer}.
		// However, some codes might have underscores.
		// A safer check is to see if any valid prefix is the START of this string
		// followed immediately by an underscore.
		for p := range prefixes {
			if strings.HasPrefix(baseName, p+"_") {
				return true
			}
		}
	}

	return false
}
