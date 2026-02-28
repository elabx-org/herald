package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/config"
)

func TestAuthMiddlewareBlocks(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 0
	cfg.APIToken = "correct-token"
	srv := api.NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddlewareAllows(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 0
	cfg.APIToken = "correct-token"
	srv := api.NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	// Health is unauthenticated
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health status = %d, want 200", w.Code)
	}
}

func TestAuthMiddlewareWithToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 0
	cfg.APIToken = "correct-token"
	srv := api.NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	req.Header.Set("Authorization", "Bearer correct-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
