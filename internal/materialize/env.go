package materialize

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/elabx-org/herald/internal/cache"
	"github.com/elabx-org/herald/internal/resolver"
)

type Resolver interface {
	Resolve(ctx context.Context, vault, item, field string) (string, string, error)
}

type Result struct {
	Resolved   int
	CacheHits  int
	Failed     int
	DurationMs int64
}

type EnvMaterializer struct {
	store         *cache.Store
	manager       Resolver
	defaultPolicy string
	defaultTTL    int
}

func NewEnvMaterializer(store *cache.Store, mgr Resolver, defaultPolicy string, defaultTTL int) *EnvMaterializer {
	return &EnvMaterializer{store: store, manager: mgr, defaultPolicy: defaultPolicy, defaultTTL: defaultTTL}
}

func (m *EnvMaterializer) Materialize(ctx context.Context, stack string, refs map[string]*resolver.SecretRef, outPath string) (*Result, error) {
	start := time.Now()
	result := &Result{}
	resolved := make(map[string]string)

	for varName, ref := range refs {
		cacheKey := fmt.Sprintf("%s/%s/%s", ref.Vault, ref.Item, ref.Field)

		// Try cache first
		if entry, err := m.store.Get(cacheKey); err == nil {
			resolved[varName] = entry.Value
			result.CacheHits++
			continue
		}

		// Resolve from provider
		val, providerName, err := m.manager.Resolve(ctx, ref.Vault, ref.Item, ref.Field)
		if err != nil {
			result.Failed++
			return nil, fmt.Errorf("resolve %s (%s): %w", varName, ref.Raw, err)
		}

		// Cache the result
		m.store.Set(cacheKey, &cache.Entry{
			Value:     val,
			Provider:  providerName,
			Policy:    m.defaultPolicy,
			ExpiresAt: time.Now().Add(time.Duration(m.defaultTTL) * time.Second),
		})

		resolved[varName] = val
		result.Resolved++
	}

	// Write env file
	if err := writeEnvFile(outPath, resolved); err != nil {
		return nil, fmt.Errorf("write env file: %w", err)
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result, nil
}

func writeEnvFile(path string, vars map[string]string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	for k, v := range vars {
		fmt.Fprintf(f, "%s=%s\n", k, v)
	}
	return nil
}
