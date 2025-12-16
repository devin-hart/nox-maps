package maps

import (
	"encoding/json"
	"os"
	"strings"
)

var ZoneFileMap = make(map[string]string)

func LoadZoneConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var rawMap map[string]string
	if err := json.NewDecoder(file).Decode(&rawMap); err != nil {
		return err
	}

	for k, v := range rawMap {
		ZoneFileMap[strings.ToLower(k)] = v
	}
	return nil
}

func GetZoneFileName(zoneName string) string {
	if val, ok := ZoneFileMap[strings.ToLower(zoneName)]; ok {
		return val
	}
	return ""
}