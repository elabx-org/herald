package core_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elabx-org/herald/internal/core"
	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/providers"
)

type countingProvider struct {
	calls int64
	err   error
}

func (p *countingProvider) Name() string  { return "counting" }
func (p *countingProvider) Type() string  { return "test" }
func (p *countingProvider) Priority() int { return 0 }
func (p *countingProvider) Close() error  { return nil }
func (p *countingProvider) Healthy(_ context.Context) (bool, int64, error) {
	return p.err == nil, 0, nil
}
func (p *countingProvider) ListVaults(_ context.Context) ([]providers.Vault, error) {
	return nil, nil
}
func (p *countingProvider) ListItems(_ context.Context, _ string) ([]providers.Item, error) {
	return nil, nil
}
func (p *countingProvider) ListFields(_ context.Context, _, _ string) ([]providers.Field, error) {
	return nil, nil
}
func (p *countingProvider) Resolve(_ context.Context, _, _, _ string) (string, error) {
	atomic.AddInt64(&p.calls, 1)
	if p.err != nil {
		return "", p.err
	}
	return "secret-value", nil
}

func TestManager_Singleflight(t *testing.T) {
	store, _ := cache.Open(t.TempDir()+"/c.db", "pass")
	defer store.Close()
	cp := &countingProvider{}
	mgr := core.NewManager(store, []providers.Provider{cp}, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.Resolve(context.Background(), "V", "I", "F")
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&cp.calls) > 1 {
		t.Errorf("singleflight failed: provider called %d times", cp.calls)
	}
}

func TestManager_StaleFallback(t *testing.T) {
	store, _ := cache.Open(t.TempDir()+"/c.db", "pass")
	defer store.Close()
	// Pre-populate with expired entry
	store.Set(context.Background(), "counting/V/I/F", cache.Entry{
		Value: "stale-value", Provider: "counting",
		ExpiresAt: time.Now().Add(-time.Second),
	})
	cp := &countingProvider{err: errors.New("provider down")}
	mgr := core.NewManager(store, []providers.Provider{cp}, time.Hour)

	val, err := mgr.Resolve(context.Background(), "V", "I", "F")
	if err != nil {
		t.Fatalf("expected stale value, got error: %v", err)
	}
	if val != "stale-value" {
		t.Errorf("expected stale-value, got %q", val)
	}
}
