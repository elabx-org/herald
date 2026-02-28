package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elabx-org/herald/internal/provider"
)

func TestConnectProviderResolve(t *testing.T) {
	// Mock Connect Server responses
	mux := http.NewServeMux()

	// GET /v1/vaults — list vaults
	mux.HandleFunc("/v1/vaults", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "vault-id-123", "name": "HomeLabVault"},
		})
	})

	// GET /v1/vaults/{vaultID}/items — list items
	mux.HandleFunc("/v1/vaults/vault-id-123/items", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "item-id-456", "title": "postgres-myapp"},
		})
	})

	// GET /v1/vaults/{vaultID}/items/{itemID} — get item fields
	mux.HandleFunc("/v1/vaults/vault-id-123/items/item-id-456", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "item-id-456",
			"title": "postgres-myapp",
			"fields": []map[string]interface{}{
				{"label": "password", "value": "super-secret-pass"},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := provider.NewConnectProvider("connect", srv.URL, "test-token", 1)

	val, err := p.Resolve(context.Background(), "HomeLabVault", "postgres-myapp", "password")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if val != "super-secret-pass" {
		t.Errorf("val = %q, want super-secret-pass", val)
	}
}

func TestConnectProviderHealthy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/vaults", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]interface{}{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := provider.NewConnectProvider("connect", srv.URL, "test-token", 1)
	ok, _, err := p.Healthy(context.Background())
	if err != nil || !ok {
		t.Errorf("Healthy() = %v, %v; want true, nil", ok, err)
	}
}
