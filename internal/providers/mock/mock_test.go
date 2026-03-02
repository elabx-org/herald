package mock_test

import (
	"context"
	"os"
	"testing"

	"github.com/elabx-org/herald/internal/providers/mock"
)

func TestMock_Resolve(t *testing.T) {
	yamlContent := `
secrets:
  HomeLab:
    myapp:
      db_password: "s3cr3t"
      api_key: "abc123"
`
	f, _ := os.CreateTemp("", "secrets*.yaml")
	f.WriteString(yamlContent)
	f.Close()
	defer os.Remove(f.Name())

	p, err := mock.New("test-mock", f.Name(), 99)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	val, err := p.Resolve(context.Background(), "HomeLab", "myapp", "db_password")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "s3cr3t" {
		t.Errorf("expected s3cr3t, got %q", val)
	}
}

func TestMock_ResolveNotFound(t *testing.T) {
	f, _ := os.CreateTemp("", "secrets*.yaml")
	f.WriteString("secrets: {}")
	f.Close()
	defer os.Remove(f.Name())

	p, _ := mock.New("test-mock", f.Name(), 99)
	_, err := p.Resolve(context.Background(), "Vault", "item", "field")
	if err == nil {
		t.Error("expected error for missing secret")
	}
}
