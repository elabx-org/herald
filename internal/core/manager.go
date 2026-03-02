package core

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/providers"
)

type Manager struct {
	cache      *cache.Store
	providers  []providers.Provider
	defaultTTL time.Duration
	sf         singleflight.Group
}

func NewManager(c *cache.Store, ps []providers.Provider, defaultTTL time.Duration) *Manager {
	return &Manager{cache: c, providers: ps, defaultTTL: defaultTTL}
}

func (m *Manager) Resolve(ctx context.Context, vault, item, field string) (string, error) {
	key := m.cacheKey(vault, item, field)
	if e, found, err := m.cache.Get(ctx, key); err == nil && found {
		return e.Value, nil
	}

	val, err, _ := m.sf.Do(key, func() (interface{}, error) {
		return m.fetchFromProvider(ctx, vault, item, field)
	})
	if err != nil {
		// Stale fallback
		if e, found, serr := m.cache.GetStale(ctx, key); serr == nil && found {
			return e.Value, nil
		}
		return "", err
	}
	return val.(string), nil
}

func (m *Manager) fetchFromProvider(ctx context.Context, vault, item, field string) (string, error) {
	var lastErr error
	for _, p := range m.providers {
		val, err := p.Resolve(ctx, vault, item, field)
		if err != nil {
			lastErr = err
			continue
		}
		_ = m.cache.Set(ctx, m.providerKey(p, vault, item, field), cache.Entry{
			Value:     val,
			Provider:  p.Name(),
			ExpiresAt: time.Now().Add(m.defaultTTL),
		})
		return val, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no providers available")
}

func (m *Manager) cacheKey(vault, item, field string) string {
	if len(m.providers) > 0 {
		return m.providerKey(m.providers[0], vault, item, field)
	}
	return fmt.Sprintf("_/%s/%s/%s", vault, item, field)
}

func (m *Manager) providerKey(p providers.Provider, vault, item, field string) string {
	return fmt.Sprintf("%s/%s/%s/%s", p.Name(), vault, item, field)
}
