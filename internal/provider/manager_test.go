package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elabx-org/herald/internal/provider"
)

type mockProvider struct {
	name    string
	value   string
	err     error
	healthy bool
}

func (m *mockProvider) Name() string     { return m.name }
func (m *mockProvider) Priority() int    { return 1 }
func (m *mockProvider) Type() string     { return "mock" }
func (m *mockProvider) Resolve(ctx context.Context, vault, item, field string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.value, nil
}
func (m *mockProvider) Healthy(ctx context.Context) (bool, int64, error) {
	return m.healthy, 5, nil
}

func TestManagerResolvesWithPrimary(t *testing.T) {
	mgr := provider.NewManager([]provider.Provider{
		&mockProvider{name: "primary", value: "secret-value", healthy: true},
	})

	val, name, err := mgr.Resolve(context.Background(), "vault", "item", "field")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if val != "secret-value" {
		t.Errorf("val = %q, want secret-value", val)
	}
	if name != "primary" {
		t.Errorf("provider name = %q, want primary", name)
	}
}

func TestManagerFallsBackOnError(t *testing.T) {
	mgr := provider.NewManager([]provider.Provider{
		&mockProvider{name: "primary", err: errors.New("unavailable"), healthy: false},
		&mockProvider{name: "fallback", value: "fallback-value", healthy: true},
	})

	val, name, err := mgr.Resolve(context.Background(), "vault", "item", "field")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if val != "fallback-value" {
		t.Errorf("val = %q, want fallback-value", val)
	}
	if name != "fallback" {
		t.Errorf("provider name = %q, want fallback", name)
	}
}

func TestManagerAllFailsReturnsError(t *testing.T) {
	mgr := provider.NewManager([]provider.Provider{
		&mockProvider{name: "primary", err: errors.New("down"), healthy: false},
	})

	_, _, err := mgr.Resolve(context.Background(), "vault", "item", "field")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
