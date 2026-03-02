package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elabx-org/herald/internal/api"
)

func TestPing(t *testing.T) {
	srv := api.NewServer(api.Options{})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestProtectedWithoutToken(t *testing.T) {
	srv := api.NewServer(api.Options{APIToken: "secret"})
	req := httptest.NewRequest(http.MethodGet, "/v2/inventory", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
