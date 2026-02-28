package materialize

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/elabx-org/herald/internal/cache"
	"github.com/elabx-org/herald/internal/resolver"
	"github.com/rs/zerolog/log"
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

// Materialize resolves all op:// refs in envContent and returns the complete
// resolved env content (non-secret lines preserved). If outPath is non-empty,
// the resolved content is also written to that file.
func (m *EnvMaterializer) Materialize(ctx context.Context, stack string, refs map[string]*resolver.SecretRef, envContent string, outPath string) (string, *Result, error) {
	start := time.Now()
	result := &Result{}
	resolvedVals := make(map[string]string)

	for varName, ref := range refs {
		cacheKey := fmt.Sprintf("%s/%s/%s", ref.Vault, ref.Item, ref.Field)

		if m.store != nil {
			if entry, err := m.store.Get(cacheKey); err == nil {
				resolvedVals[varName] = entry.Value
				result.CacheHits++
				continue
			}
		}

		val, providerName, err := m.manager.Resolve(ctx, ref.Vault, ref.Item, ref.Field)
		if err != nil {
			result.Failed++
			return "", result, fmt.Errorf("resolve %s (%s): %w", varName, ref.Raw, err)
		}

		if m.store != nil {
			if err := m.store.Set(cacheKey, &cache.Entry{
				Value:     val,
				Provider:  providerName,
				Policy:    m.defaultPolicy,
				ExpiresAt: time.Now().Add(time.Duration(m.defaultTTL) * time.Second),
			}); err != nil {
				log.Warn().Err(err).Str("key", cacheKey).Msg("materialize: cache write failed")
			}
		}

		resolvedVals[varName] = val
		result.Resolved++
	}

	// Build complete resolved env content
	content := resolver.ResolveEnvContent(envContent, resolvedVals)

	// Write to file if path specified
	if outPath != "" {
		if err := writeFile(outPath, content); err != nil {
			return "", result, fmt.Errorf("write env file: %w", err)
		}
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return content, result, nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
