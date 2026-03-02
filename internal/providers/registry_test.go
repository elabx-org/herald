package providers_test

import (
	"context"
	"testing"

	"github.com/elabx-org/herald/internal/providers"
)

type stubProvider struct {
	name     string
	priority int
}

func (s *stubProvider) Name() string  { return s.name }
func (s *stubProvider) Type() string  { return "stub" }
func (s *stubProvider) Priority() int { return s.priority }
func (s *stubProvider) Resolve(_ context.Context, _, _, _ string) (string, error) {
	return "val", nil
}
func (s *stubProvider) Healthy(_ context.Context) (bool, int64, error) { return true, 1, nil }
func (s *stubProvider) ListVaults(_ context.Context) ([]providers.Vault, error) {
	return nil, nil
}
func (s *stubProvider) ListItems(_ context.Context, _ string) ([]providers.Item, error) {
	return nil, nil
}
func (s *stubProvider) ListFields(_ context.Context, _, _ string) ([]providers.Field, error) {
	return nil, nil
}
func (s *stubProvider) Close() error { return nil }

func TestRegistry_PriorityOrdering(t *testing.T) {
	r := providers.NewRegistry()
	r.Register(&stubProvider{"b", 5})
	r.Register(&stubProvider{"a", 1})
	r.Register(&stubProvider{"c", 10})

	ordered := r.Ordered()
	if ordered[0].Name() != "a" || ordered[1].Name() != "b" || ordered[2].Name() != "c" {
		t.Errorf("unexpected order: %v %v %v", ordered[0].Name(), ordered[1].Name(), ordered[2].Name())
	}
}
