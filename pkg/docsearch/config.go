package docsearch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the application configuration for docsearch-gui.
type Config struct {
	Server  ServerConfig  `json:"server"`
	Hotkey  HotkeyConfig  `json:"hotkey"`
	Indexes []IndexConfig `json:"indexes"`
	LLM     LLMConfig     `json:"llm"`
}

// ServerConfig defines the HTTP server port and host address.
type ServerConfig struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

// HotkeyConfig defines the hotkey activation settings.
type HotkeyConfig struct {
	Enabled    bool `json:"enabled"`
	IntervalMS int  `json:"interval_ms"`
}

// IndexConfig defines a Bleve search index target.
type IndexConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	DefaultSelected bool   `json:"default_selected"`
}

// LLMConfig defines LLM-based query expansion and analysis settings.
type LLMConfig struct {
	QueryExpansion bool   `json:"query_expansion"`
	Model          string `json:"model"`
}

// DefaultConfig returns a Config initialized with default settings.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 18080,
			Host: "127.0.0.1",
		},
		Hotkey: HotkeyConfig{
			Enabled:    true,
			IntervalMS: 400,
		},
		Indexes: []IndexConfig{},
		LLM: LLMConfig{
			QueryExpansion: true,
			Model:          "gemini-2.5-flash",
		},
	}
}

// LoadConfig loads the configuration from the specified JSON file path.
// If path is empty or the file does not exist, it returns the default configuration.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON from %s: %w", path, err)
	}

	// Apply fallbacks for zero values
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 18080
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Hotkey.IntervalMS <= 0 {
		cfg.Hotkey.IntervalMS = 400
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = "gemini-2.5-flash"
	}

	return cfg, nil
}

// SaveConfig saves the configuration to the specified JSON file path.
func SaveConfig(path string, cfg *Config) error {
	if path == "" {
		return fmt.Errorf("config file path cannot be empty")
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}

	return nil
}
