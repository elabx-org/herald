package provisioner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A Connect item as 1Password stores it: version, sections, URLs and built-in fields with
// purposes. Herald must send all of that back when it adds a field.
const fullItem = `{
  "id": "item1", "title": "homelab-nodes", "category": "LOGIN", "version": 7,
  "vault": {"id": "v1"}, "tags": ["homelab"],
  "urls": [{"href": "http://10.0.0.15:5380", "primary": true}],
  "sections": [{"id": "s1", "label": "keys"}],
  "fields": [
    {"id": "username", "type": "STRING", "purpose": "USERNAME", "label": "username", "value": "admin"},
    {"id": "password", "type": "CONCEALED", "purpose": "PASSWORD", "label": "password", "value": "pw"},
    {"id": "notesPlain", "type": "STRING", "purpose": "NOTES", "label": "notesPlain"},
    {"id": "existing_key", "type": "CONCEALED", "label": "existing_key", "value": "old", "section": {"id": "s1"}}
  ]
}`

func fakeConnect(t *testing.T, putStatus int, putBody string, gotPut *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults":
			io.WriteString(w, `[{"id":"v1","name":"HomeLab"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults/v1/items":
			io.WriteString(w, `[{"id":"item1","title":"homelab-nodes"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults/v1/items/item1":
			io.WriteString(w, fullItem)
		case r.Method == http.MethodPut && r.URL.Path == "/v1/vaults/v1/items/item1":
			body, _ := io.ReadAll(r.Body)
			if gotPut != nil {
				_ = json.Unmarshal(body, gotPut)
			}
			w.WriteHeader(putStatus)
			if putBody != "" {
				io.WriteString(w, putBody)
			} else {
				w.Write(body)
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func fieldByID(item map[string]any, id string) map[string]any {
	for _, f := range item["fields"].([]any) {
		if m := f.(map[string]any); m["id"] == id {
			return m
		}
	}
	return nil
}

func TestConnectUpsertPreservesItem(t *testing.T) {
	var put map[string]any
	srv := fakeConnect(t, http.StatusOK, "", &put)
	defer srv.Close()
	p := NewConnectProvisioner(srv.URL, "tok")
	res, err := p.Provision(context.Background(), ProvisionRequest{Vault: "HomeLab", Item: "homelab-nodes",
		Fields: map[string]FieldSpec{"new_key": {Value: "v", Concealed: true}, "existing_key": {Value: "ignored"}}})
	if err != nil {
		t.Fatal(err)
	}
	if put["version"] != float64(7) || put["tags"] == nil || put["urls"] == nil || put["sections"] == nil {
		t.Errorf("item metadata dropped on PUT: %v", put)
	}
	if f := fieldByID(put, "password"); f == nil || f["purpose"] != "PASSWORD" {
		t.Errorf("built-in field purpose dropped: %v", f)
	}
	if f := fieldByID(put, "existing_key"); f["value"] != "old" || f["section"] == nil {
		t.Errorf("existing field changed without overwrite, or its section dropped: %v", f)
	}
	if f := fieldByID(put, "new_key"); f == nil || f["type"] != "CONCEALED" || f["value"] != "v" {
		t.Errorf("new field not added: %v", f)
	}
	if res.Refs["new_key"] != "op://HomeLab/homelab-nodes/new_key" || res.ItemID != "item1" {
		t.Errorf("unexpected result %+v", res)
	}
}

func TestConnectUpsertOverwrite(t *testing.T) {
	var put map[string]any
	srv := fakeConnect(t, http.StatusOK, "", &put)
	defer srv.Close()
	_, err := NewConnectProvisioner(srv.URL, "tok").Provision(context.Background(), ProvisionRequest{
		Vault: "HomeLab", Item: "homelab-nodes", Fields: map[string]FieldSpec{"existing_key": {Value: "rotated", Overwrite: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if f := fieldByID(put, "existing_key"); f["value"] != "rotated" || f["section"] == nil {
		t.Errorf("overwrite failed: %v", f)
	}
}

func TestConnectUpsertNoChangeNoPut(t *testing.T) {
	var put map[string]any
	srv := fakeConnect(t, http.StatusOK, "", &put)
	defer srv.Close()
	_, err := NewConnectProvisioner(srv.URL, "tok").Provision(context.Background(), ProvisionRequest{
		Vault: "HomeLab", Item: "homelab-nodes", Fields: map[string]FieldSpec{"existing_key": {Value: "x"}}})
	if err != nil || put != nil {
		t.Errorf("expected no PUT when nothing changes (err=%v, put=%v)", err, put)
	}
}

func TestConnectErrorIncludesMessage(t *testing.T) {
	srv := fakeConnect(t, http.StatusBadRequest, `{"status":400,"message":"Validation: (validateVaultItem failed)"}`, nil)
	defer srv.Close()
	_, err := NewConnectProvisioner(srv.URL, "tok").Provision(context.Background(), ProvisionRequest{
		Vault: "HomeLab", Item: "homelab-nodes", Fields: map[string]FieldSpec{"k": {Value: "v"}}})
	if err == nil || !strings.Contains(err.Error(), "HTTP 400: Validation: (validateVaultItem failed)") {
		t.Errorf("error should carry Connect's message, got %v", err)
	}
}
