package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/bbolt"
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

// containsItem checks if a cache key (provider/vault/item/field) matches the given item/vault.
func containsItem(key, vault, item string) bool {
	// key format: provider/vault/item/field
	parts := strings.SplitN(key, "/", 4)
	if len(parts) < 3 {
		return false
	}
	if vault != "" && parts[1] != vault {
		return false
	}
	return parts[2] == item
}

// ProviderNames returns the names of configured providers.
func (m *Manager) ProviderNames() []string {
	names := make([]string, len(m.providers))
	for i, p := range m.providers {
		names[i] = p.Name()
	}
	return names
}

// CacheCount returns the number of entries in the cache.
func (m *Manager) CacheCount() int {
	return m.cache.Count()
}

// FlushAll clears the entire cache.
func (m *Manager) FlushAll() error {
	return m.cache.Flush()
}

// FlushItem deletes all cache entries containing the given item (optionally scoped to vault).
// Returns the number of keys deleted. Uses bbolt cursor scan — O(n) over cache size.
func (m *Manager) FlushItem(ctx context.Context, vault, item string) int {
	db := m.cache.DB()
	var keys [][]byte
	db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("secrets"))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, _ []byte) error {
			key := string(k)
			// key format: provider/vault/item/field
			if containsItem(key, vault, item) {
				keys = append(keys, append([]byte{}, k...))
			}
			return nil
		})
	})
	for _, k := range keys {
		db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte("secrets"))
			if b == nil {
				return nil
			}
			return b.Delete(k)
		})
	}
	return len(keys)
}
