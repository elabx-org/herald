package provider

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// Manager holds an ordered list of providers and implements fallback resolution.
type Manager struct {
	providers []Provider
}

func NewManager(providers []Provider) *Manager {
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Priority() < providers[j].Priority()
	})
	return &Manager{providers: providers}
}

// Resolve attempts each provider in priority order, returning the first success.
// Returns (value, providerName, error).
func (m *Manager) Resolve(ctx context.Context, vault, item, field string) (string, string, error) {
	var lastErr error
	for _, p := range m.providers {
		val, err := p.Resolve(ctx, vault, item, field)
		if err != nil {
			lastErr = err
			continue
		}
		return val, p.Name(), nil
	}
	if lastErr != nil {
		return "", "", fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return "", "", fmt.Errorf("no providers configured")
}

// Health returns the status of all providers.
func (m *Manager) Health(ctx context.Context) []ProviderHealth {
	results := make([]ProviderHealth, len(m.providers))
	for i, p := range m.providers {
		ok, latency, err := p.Healthy(ctx)
		h := ProviderHealth{Name: p.Name(), Healthy: ok, LatencyMs: latency}
		if err != nil {
			h.Error = err.Error()
		}
		if rl, ok := p.(interface{ RateLimitedSince() *time.Time }); ok {
			h.RateLimitedSince = rl.RateLimitedSince()
		}
		results[i] = h
	}
	return results
}

// Names returns the names of all configured providers.
func (m *Manager) Names() []string {
	names := make([]string, len(m.providers))
	for i, p := range m.providers {
		names[i] = p.Name()
	}
	return names
}

type ProviderHealth struct {
	Name             string
	Healthy          bool
	LatencyMs        int64
	Error            string
	RateLimitedSince *time.Time
}
