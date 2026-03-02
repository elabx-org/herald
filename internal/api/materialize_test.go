package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/core"
	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/providers"
)

type fixedProvider struct{ val string }

func (p *fixedProvider) Name() string  { return "fixed" }
func (p *fixedProvider) Type() string  { return "test" }
func (p *fixedProvider) Priority() int { return 0 }
func (p *fixedProvider) Close() error  { return nil }
func (p *fixedProvider) Healthy(_ context.Context) (bool, int64, error) { return true, 0, nil }
func (p *fixedProvider) ListVaults(_ context.Context) ([]providers.Vault, error) {
	return nil, nil
}
func (p *fixedProvider) ListItems(_ context.Context, _ string) ([]providers.Item, error) {
	return nil, nil
}
func (p *fixedProvider) ListFields(_ context.Context, _, _ string) ([]providers.Field, error) {
	return nil, nil
}
func (p *fixedProvider) Resolve(_ context.Context, _, _, _ string) (string, error) {
	return p.val, nil
}

func TestMaterialize_ResolvesRefs(t *testing.T) {
	store, _ := cache.Open(t.TempDir()+"/c.db", "pass")
	defer store.Close()
	fp := &fixedProvider{val: "resolved-secret"}
	mgr := core.NewManager(store, []providers.Provider{fp}, time.Hour)

	srv := api.NewServer(api.Options{Manager: mgr})
	body, _ := json.Marshal(map[string]string{
		"stack":       "teststack",
		"env_content": "DB_PASS=herald://V/I/F\n",
	})
	req := httptest.NewRequest(http.MethodPost, "/v2/materialize/env", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if content, ok := resp["content"].(string); !ok || content != "DB_PASS=resolved-secret\n" {
		t.Errorf("unexpected content: %v", resp["content"])
	}
}
