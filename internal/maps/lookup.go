package maps

import (
	"bufio"
	"os"
	"strings"
)

// LoadZoneLookup reads map_keys.ini and returns a map of "Long Name" -> "shortname"
func LoadZoneLookup(path string) (map[string]string, error) {
	lookup := make(map[string]string)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments or empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}

		// Format is: Long Name = shortname
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			longName := strings.TrimSpace(parts[0])
			shortName := strings.TrimSpace(parts[1])
			
			// Normalize keys to lowercase for safer matching
			lookup[strings.ToLower(longName)] = shortName
		}
	}

	return lookup, nil
}