package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Marker struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label"`
	Color string  `json:"color"` // "red", "blue", "green", "yellow", "purple"
	Shape string  `json:"shape"` // "circle", "square", "triangle", "diamond", "star"
}

type Config struct {
	EQPath  string              `json:"eq_path"`
	Markers map[string][]Marker `json:"markers"` // zone name -> markers
}

func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "nox-maps")
	os.MkdirAll(configDir, 0755)
	return filepath.Join(configDir, "config.json")
}

func Load() *Config {
	configPath := GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &Config{
			EQPath:  "",
			Markers: make(map[string][]Marker),
		}
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{
			EQPath:  "",
			Markers: make(map[string][]Marker),
		}
	}

	// Initialize markers map if nil
	if cfg.Markers == nil {
		cfg.Markers = make(map[string][]Marker)
	}

	return &cfg
}

func (c *Config) Save() error {
	configPath := GetConfigPath()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}
