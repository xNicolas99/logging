package config

import (
	"os"
	"testing"
)

func TestConfig(t *testing.T) {
	tmpFile := "test_config.json"
	defer os.Remove(tmpFile)

	// Test DefaultConfig
	cfg := DefaultConfig()
	if cfg.Interval != 3600 {
		t.Errorf("Expected default interval 3600, got %d", cfg.Interval)
	}

	// Test SaveConfig
	cfg.Interval = 1234
	if err := SaveConfig(tmpFile, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test LoadConfig
	loaded, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.Interval != 1234 {
		t.Errorf("Expected interval 1234, got %d", loaded.Interval)
	}
	if len(loaded.Targets) != len(cfg.Targets) {
		t.Errorf("Targets mismatch")
	}
}
