package config_test

import (
	"os"
	"testing"

	"github.com/elabx-org/herald/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8765 {
		t.Errorf("expected port 8765, got %d", cfg.Port)
	}
	if cfg.Cache.DataPath != "/data/cache.db" {
		t.Errorf("unexpected cache path: %s", cfg.Cache.DataPath)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	os.Setenv("HERALD_PORT", "9000")
	defer os.Unsetenv("HERALD_PORT")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
}
