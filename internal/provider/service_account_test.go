package provider_test

import (
	"os"
	"testing"

	"github.com/elabx-org/herald/internal/provider"
)

func TestServiceAccountProviderMissingToken(t *testing.T) {
	p, err := provider.NewServiceAccountProvider("sa", "", 2)
	if err == nil {
		t.Fatal("expected error for empty token")
	}
	_ = p
}

func TestServiceAccountProviderName(t *testing.T) {
	token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN")
	if token == "" {
		t.Skip("OP_SERVICE_ACCOUNT_TOKEN not set, skipping integration test")
	}
	p, err := provider.NewServiceAccountProvider("sa", token, 2)
	if err != nil {
		t.Fatalf("NewServiceAccountProvider() error = %v", err)
	}
	if p.Name() != "sa" {
		t.Errorf("Name() = %q, want sa", p.Name())
	}
}

// Verify the provider satisfies the Provider interface at compile time
func TestServiceAccountImplementsProvider(t *testing.T) {
	var _ provider.Provider = (*provider.ServiceAccountProvider)(nil)
}
