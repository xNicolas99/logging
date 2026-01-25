package config

import (
	"testing"

	"github.com/jules/http-monitor/internal/model"
)

func TestEnsureMandatoryTargets(t *testing.T) {
	cfg := &Config{
		Targets: []model.Target{},
	}

	// First run: should add targets
	if !cfg.EnsureMandatoryTargets() {
		t.Error("Expected EnsureMandatoryTargets to return true on empty config")
	}

	if len(cfg.Targets) != 4 {
		t.Errorf("Expected 4 targets, got %d", len(cfg.Targets))
	}

	// Verify GitHub is present
	found := false
	for _, target := range cfg.Targets {
		if target.Name == "GitHub" {
			found = true
			break
		}
	}
	if !found {
		t.Error("GitHub target not found")
	}

	// Second run: should not add anything
	if cfg.EnsureMandatoryTargets() {
		t.Error("Expected EnsureMandatoryTargets to return false on already populated config")
	}

	if len(cfg.Targets) != 4 {
		t.Errorf("Expected 4 targets, got %d", len(cfg.Targets))
	}
}
