package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/sync/singleflight"

	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/providers"
)

// ProviderMeta holds non-secret metadata about a configured provider.
type ProviderMeta struct {
	URL    string // plaintext Connect URL or mock path; empty for SDK
	Source string // "env" or "db"
}

// ProviderStatus holds the last health-check result for a single provider.
type ProviderStatus struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Priority  int       `json:"priority"`
	Healthy   bool      `json:"healthy"`
	LatencyMs int64     `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	URL       string    `json:"url,omitempty"`
	Source    string    `json:"source"` // "env" or "db"
}

type Manager struct {
	cache      *cache.Store
	providers  []providers.Provider
	defaultTTL time.Duration
	sf         singleflight.Group

	healthMu    sync.RWMutex
	healthCache []ProviderStatus

	providerMu sync.RWMutex
	meta       map[string]ProviderMeta // keyed by provider name
}

func NewManager(c *cache.Store, ps []providers.Provider, defaultTTL time.Duration) *Manager {
	return &Manager{cache: c, providers: ps, defaultTTL: defaultTTL, meta: make(map[string]ProviderMeta)}
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
	m.providerMu.RLock()
	provs := make([]providers.Provider, len(m.providers))
	copy(provs, m.providers)
	m.providerMu.RUnlock()

	var lastErr error
	for _, p := range provs {
		val, err := p.Resolve(ctx, vault, item, field)
		if err != nil {
			lastErr = err
			continue
		}
		_ = m.cache.Set(ctx, fmt.Sprintf("%s/%s/%s", vault, item, field), cache.Entry{
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
	return fmt.Sprintf("%s/%s/%s", vault, item, field)
}

// containsItem checks if a cache key (vault/item/field) matches the given item/vault.
func containsItem(key, vault, item string) bool {
	// key format: vault/item/field
	parts := strings.SplitN(key, "/", 3)
	if len(parts) < 2 {
		return false
	}
	if vault != "" && parts[0] != vault {
		return false
	}
	return parts[1] == item
}

// ProviderNames returns the names of configured providers.
func (m *Manager) ProviderNames() []string {
	m.providerMu.RLock()
	names := make([]string, len(m.providers))
	for i, p := range m.providers {
		names[i] = p.Name()
	}
	m.providerMu.RUnlock()
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
		// Enrich with metadata (URL/Source not stored in health cache)
		m.providerMu.RLock()
		for i := range out {
			meta := m.meta[out[i].Name]
			out[i].URL = meta.URL
			out[i].Source = meta.Source
			if out[i].Source == "" {
				out[i].Source = "env"
			}
		}
		m.providerMu.RUnlock()
		return out
	}

	// No check run yet — return basic info.
	m.providerMu.RLock()
	out := make([]ProviderStatus, len(m.providers))
	for i, p := range m.providers {
		out[i] = ProviderStatus{
			Name:     p.Name(),
			Type:     p.Type(),
			Priority: p.Priority(),
			URL:      m.meta[p.Name()].URL,
			Source:   m.meta[p.Name()].Source,
		}
		if out[i].Source == "" {
			out[i].Source = "env"
		}
	}
	m.providerMu.RUnlock()
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
	m.providerMu.RLock()
	provs := make([]providers.Provider, len(m.providers))
	copy(provs, m.providers)
	m.providerMu.RUnlock()

	results := make([]ProviderStatus, 0, len(provs))
	for _, p := range provs {
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

// SetMeta records source metadata for a provider (call during startup for env providers).
func (m *Manager) SetMeta(name string, meta ProviderMeta) {
	m.providerMu.Lock()
	defer m.providerMu.Unlock()
	m.meta[name] = meta
}

// AddProvider adds a new provider and activates it immediately.
// Returns error if a provider with the same name already exists.
func (m *Manager) AddProvider(p providers.Provider, meta ProviderMeta) error {
	m.providerMu.Lock()
	defer m.providerMu.Unlock()
	for _, existing := range m.providers {
		if existing.Name() == p.Name() {
			return fmt.Errorf("provider %q already exists", p.Name())
		}
	}
	m.providers = append(m.providers, p)
	sort.Slice(m.providers, func(i, j int) bool {
		return m.providers[i].Priority() < m.providers[j].Priority()
	})
	m.meta[p.Name()] = meta
	return nil
}

// UpdateProvider replaces an existing provider by name, or adds it if not found.
func (m *Manager) UpdateProvider(p providers.Provider, meta ProviderMeta) error {
	m.providerMu.Lock()
	defer m.providerMu.Unlock()
	for i, existing := range m.providers {
		if existing.Name() == p.Name() {
			m.providers[i] = p
			sort.Slice(m.providers, func(i, j int) bool {
				return m.providers[i].Priority() < m.providers[j].Priority()
			})
			m.meta[p.Name()] = meta
			return nil
		}
	}
	// Not found — add it (handles env→db override)
	m.providers = append(m.providers, p)
	sort.Slice(m.providers, func(i, j int) bool {
		return m.providers[i].Priority() < m.providers[j].Priority()
	})
	m.meta[p.Name()] = meta
	return nil
}

// RemoveProvider deactivates a provider by name.
func (m *Manager) RemoveProvider(name string) error {
	m.providerMu.Lock()
	defer m.providerMu.Unlock()
	for i, p := range m.providers {
		if p.Name() == name {
			m.providers = append(m.providers[:i], m.providers[i+1:]...)
			delete(m.meta, name)
			return nil
		}
	}
	return fmt.Errorf("provider %q not found", name)
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
			// key format: vault/item/field
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
