package komodo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elabx-org/herald/internal/komodo"
)

func TestDeployStack(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/execute/DeployStack", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := komodo.NewClient(srv.URL, "api-key", "api-secret")
	err := client.DeployStack(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("DeployStack() error = %v", err)
	}
}

func TestSendAlert(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/write/SendAlert", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := komodo.NewClient(srv.URL, "api-key", "api-secret")
	err := client.SendAlert(context.Background(), "warning", "test message")
	if err != nil {
		t.Fatalf("SendAlert() error = %v", err)
	}
}
