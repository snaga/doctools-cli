package docsearch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_DefaultWhenNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does_not_exist.json")

	cfg, err := LoadConfig(nonExistent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 18080 {
		t.Errorf("expected default port 18080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected default host '127.0.0.1', got %s", cfg.Server.Host)
	}
	if !cfg.Hotkey.Enabled {
		t.Errorf("expected hotkey enabled true")
	}
	if cfg.Hotkey.IntervalMS != 400 {
		t.Errorf("expected hotkey interval 400, got %d", cfg.Hotkey.IntervalMS)
	}
	if cfg.LLM.Model != "gemini-2.5-flash" {
		t.Errorf("expected LLM model 'gemini-2.5-flash', got %s", cfg.LLM.Model)
	}
	if !cfg.LLM.QueryExpansion {
		t.Errorf("expected query_expansion true")
	}

	// Also test empty path
	cfgEmpty, err := LoadConfig("")
	if err != nil {
		t.Fatalf("unexpected error on empty path: %v", err)
	}
	if cfgEmpty.Server.Port != 18080 {
		t.Errorf("expected default port 18080, got %d", cfgEmpty.Server.Port)
	}
}

func TestLoadConfig_CustomJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "docsearch.json")

	content := `{
  "server": {
    "port": 9090,
    "host": "0.0.0.0"
  },
  "hotkey": {
    "enabled": false,
    "interval_ms": 500
  },
  "indexes": [
    {
      "id": "rules",
      "name": "社内規程",
      "path": "C:/docs/indexes/rules.bleve",
      "default_selected": true
    }
  ],
  "llm": {
    "query_expansion": false,
    "model": "custom-model"
  }
}`

	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error on LoadConfig: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Hotkey.Enabled {
		t.Errorf("expected hotkey enabled false")
	}
	if cfg.Hotkey.IntervalMS != 500 {
		t.Errorf("expected hotkey interval 500, got %d", cfg.Hotkey.IntervalMS)
	}
	if len(cfg.Indexes) != 1 || cfg.Indexes[0].ID != "rules" {
		t.Errorf("expected 1 index with id 'rules', got %v", cfg.Indexes)
	}
	if cfg.LLM.QueryExpansion {
		t.Errorf("expected query expansion false")
	}
	if cfg.LLM.Model != "custom-model" {
		t.Errorf("expected model 'custom-model', got %s", cfg.LLM.Model)
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "sub", "docsearch.json")

	cfg := DefaultConfig()
	cfg.Server.Port = 19090

	if err := SaveConfig(cfgPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	if loaded.Server.Port != 19090 {
		t.Errorf("expected reloaded port 19090, got %d", loaded.Server.Port)
	}

	// Empty path error
	if err := SaveConfig("", cfg); err == nil {
		t.Errorf("expected error when saving to empty path")
	}

	// Nil config saves default
	nilCfgPath := filepath.Join(tmpDir, "nil_cfg.json")
	if err := SaveConfig(nilCfgPath, nil); err != nil {
		t.Fatalf("unexpected error when saving nil config: %v", err)
	}
	loadedNil, err := LoadConfig(nilCfgPath)
	if err != nil || loadedNil.Server.Port != 18080 {
		t.Errorf("expected default config loaded from saved nil config")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(badPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid json: %v", err)
	}

	if _, err := LoadConfig(badPath); err == nil {
		t.Errorf("expected error loading invalid json")
	}
}

func TestLoadConfig_ZeroValueFallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	sparsePath := filepath.Join(tmpDir, "sparse.json")
	content := `{"server": {"port": 0, "host": ""}, "hotkey": {"interval_ms": 0}, "llm": {"model": ""}}`
	if err := os.WriteFile(sparsePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write sparse json: %v", err)
	}

	cfg, err := LoadConfig(sparsePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 18080 {
		t.Errorf("expected fallback port 18080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected fallback host '127.0.0.1', got %s", cfg.Server.Host)
	}
	if cfg.Hotkey.IntervalMS != 400 {
		t.Errorf("expected fallback interval 400, got %d", cfg.Hotkey.IntervalMS)
	}
	if cfg.LLM.Model != "gemini-2.5-flash" {
		t.Errorf("expected fallback model 'gemini-2.5-flash', got %s", cfg.LLM.Model)
	}
}
