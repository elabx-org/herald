//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/core"
	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/providers"
	mockprovider "github.com/elabx-org/herald/internal/providers/mock"
)

func TestMaterialize_EndToEnd(t *testing.T) {
	tmp := t.TempDir()
	secretsFile := filepath.Join(tmp, "secrets.yaml")
	cacheFile := filepath.Join(tmp, "cache.db")

	if err := os.WriteFile(secretsFile, []byte(`
secrets:
  HomeLab:
    myapp:
      db_password: "supersecret"
      api_key: "myapikey123"
`), 0600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	cacheStore, err := cache.Open(cacheFile, "test-key-32-bytes-padded-exactly!!")
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer cacheStore.Close()

	mp, err := mockprovider.New("mock", secretsFile, 0)
	if err != nil {
		t.Fatalf("mockprovider.New: %v", err)
	}

	mgr := core.NewManager(cacheStore, []providers.Provider{mp}, 5*time.Minute)

	srv := api.NewServer(api.Options{
		Manager: mgr,
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"stack":       "test-stack",
		"env_content": "DB_PASS=op://HomeLab/myapp/db_password\nAPI_KEY=op://HomeLab/myapp/api_key\n",
	})

	resp, err := http.Post(ts.URL+"/v1/materialize/env", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/materialize/env: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Content  string `json:"content"`
		Resolved int    `json:"resolved"`
		Failed   int    `json:"failed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Resolved != 2 {
		t.Errorf("expected 2 resolved, got %d", result.Resolved)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
	if !strings.Contains(result.Content, "DB_PASS=supersecret") {
		t.Errorf("expected DB_PASS=supersecret in content, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "API_KEY=myapikey123") {
		t.Errorf("expected API_KEY=myapikey123 in content, got: %s", result.Content)
	}
	t.Logf("resolved content:\n%s", result.Content)
}

func TestHealth_EndToEnd(t *testing.T) {
	srv := api.NewServer(api.Options{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatalf("GET /v1/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPing_EndToEnd(t *testing.T) {
	srv := api.NewServer(api.Options{})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
