package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/config"
)

func newTestServer(t *testing.T) *api.Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0
	return api.NewServer(cfg, nil)
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
}
