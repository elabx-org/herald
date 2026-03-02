package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/sync/singleflight"

	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/providers"
)

// ProviderStatus holds the last health-check result for a single provider.
type ProviderStatus struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Priority  int       `json:"priority"`
	Healthy   bool      `json:"healthy"`
	LatencyMs int64     `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type Manager struct {
	cache      *cache.Store
	providers  []providers.Provider
	defaultTTL time.Duration
	sf         singleflight.Group

	healthMu    sync.RWMutex
	healthCache []ProviderStatus
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

// ProviderStatuses returns the most recent health-check results for all providers.
// If the background checker has not run yet, it returns basic info with Healthy=false.
func (m *Manager) ProviderStatuses() []ProviderStatus {
	m.healthMu.RLock()
	cached := m.healthCache
	m.healthMu.RUnlock()

	if len(cached) > 0 {
		out := make([]ProviderStatus, len(cached))
		copy(out, cached)
		return out
	}

	// No check run yet — return basic info.
	out := make([]ProviderStatus, len(m.providers))
	for i, p := range m.providers {
		out[i] = ProviderStatus{
			Name:     p.Name(),
			Type:     p.Type(),
			Priority: p.Priority(),
		}
	}
	return out
}

// StartHealthChecker launches a goroutine that checks all providers every 60 seconds.
func (m *Manager) StartHealthChecker(ctx context.Context) {
	go func() {
		m.runHealthChecks(ctx)
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.runHealthChecks(ctx)
			}
		}
	}()
}

func (m *Manager) runHealthChecks(ctx context.Context) {
	results := make([]ProviderStatus, 0, len(m.providers))
	for _, p := range m.providers {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		ok, latency, err := p.Healthy(checkCtx)
		cancel()

		status := ProviderStatus{
			Name:      p.Name(),
			Type:      p.Type(),
			Priority:  p.Priority(),
			Healthy:   ok,
			LatencyMs: latency,
			CheckedAt: time.Now(),
		}
		if err != nil {
			status.Error = err.Error()
		}
		results = append(results, status)
	}

	m.healthMu.Lock()
	m.healthCache = results
	m.healthMu.Unlock()
}

// CheckNow synchronously runs health checks against all providers, updates the
// cached results, and returns the fresh statuses.
func (m *Manager) CheckNow(ctx context.Context) []ProviderStatus {
	m.runHealthChecks(ctx)
	return m.ProviderStatuses()
}

// CacheCount returns the number of entries in the cache.
func (m *Manager) CacheCount() int {
	return m.cache.Count()
}

// CacheList returns all cache entries including stale ones.
func (m *Manager) CacheList() []cache.ListEntry {
	return m.cache.List()
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
