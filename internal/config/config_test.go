package config_test

import (
	"os"
	"testing"

	"github.com/elabx-org/herald/internal/config"
)

func TestLoadFromFile(t *testing.T) {
	content := `
server:
  host: 0.0.0.0
  port: 8765
cache:
  default_policy: memory
  default_ttl: 3600
audit:
  enabled: true
  path: /tmp/herald-test-audit.log
  retention_days: 30
`
	f, _ := os.CreateTemp("", "herald-*.yaml")
	f.WriteString(content)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := config.Load(f.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 8765 {
		t.Errorf("Server.Port = %d, want 8765", cfg.Server.Port)
	}
	if cfg.Cache.DefaultPolicy != "memory" {
		t.Errorf("Cache.DefaultPolicy = %q, want memory", cfg.Cache.DefaultPolicy)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("HERALD_API_TOKEN", "test-token-123")
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIToken != "test-token-123" {
		t.Errorf("APIToken = %q, want test-token-123", cfg.APIToken)
	}
}
