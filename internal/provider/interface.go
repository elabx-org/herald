package provider

import "context"

// Provider resolves secrets from a specific backend.
type Provider interface {
	// Name returns the unique identifier for this provider.
	Name() string
	// Priority returns the priority (lower = higher priority).
	Priority() int
	// Resolve fetches a secret value by vault/item/field.
	Resolve(ctx context.Context, vault, item, field string) (string, error)
	// Healthy checks if the provider is reachable. Returns (ok, latencyMs, error).
	Healthy(ctx context.Context) (bool, int64, error)
}
