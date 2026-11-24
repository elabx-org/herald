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

// A new LOGIN item: username/password go into the built-in fields (with their purpose), not
// into same-named custom fields next to empty built-ins.
func TestConnectCreateLoginUsesBuiltinFields(t *testing.T) {
	var posted connectItemEntry
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults":
			io.WriteString(w, `[{"id":"v1","name":"HomeLab"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults/v1/items":
			io.WriteString(w, `[]`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/vaults/v1/items":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &posted)
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	_, err := NewConnectProvisioner(srv.URL, "tok").Provision(context.Background(), ProvisionRequest{
		Vault: "HomeLab", Item: "svc", Category: "login",
		Fields: map[string]FieldSpec{"username": {Value: "svc-user"}, "password": {Value: "s3cret"}, "url": {Value: "https://x"}}})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]connectFieldEntry{}
	for _, f := range posted.Fields {
		got[f.ID] = f
	}
	if f := got["username"]; f.Purpose != "USERNAME" || f.Value != "svc-user" || f.Type != "STRING" {
		t.Errorf("username not the built-in field: %+v", f)
	}
	if f := got["password"]; f.Purpose != "PASSWORD" || f.Value != "s3cret" || f.Type != "CONCEALED" {
		t.Errorf("password not the built-in field: %+v", f)
	}
	if f := got["url"]; f.Purpose != "" {
		t.Errorf("other fields must stay plain: %+v", f)
	}
}

// An existing login whose built-ins are empty (as 1Password creates them): the values go into
// the built-ins, and no duplicate username/password fields are added.
func TestConnectUpsertFillsEmptyBuiltins(t *testing.T) {
	const emptyLogin = `{"id":"item1","title":"homelab-nodes","category":"LOGIN","version":2,"vault":{"id":"v1"},
	  "fields":[{"id":"username","type":"STRING","purpose":"USERNAME","label":"username"},
	            {"id":"password","type":"CONCEALED","purpose":"PASSWORD","label":"password"}]}`
	var put map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults":
			io.WriteString(w, `[{"id":"v1","name":"HomeLab"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults/v1/items":
			io.WriteString(w, `[{"id":"item1","title":"homelab-nodes"}]`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vaults/v1/items/item1":
			io.WriteString(w, emptyLogin)
		case r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &put)
			w.Write(body)
		}
	}))
	defer srv.Close()
	_, err := NewConnectProvisioner(srv.URL, "tok").Provision(context.Background(), ProvisionRequest{
		Vault: "HomeLab", Item: "homelab-nodes", Category: "login",
		Fields: map[string]FieldSpec{"username": {Value: "svc-user"}, "password": {Value: ""}}})
	if err != nil {
		t.Fatal(err)
	}
	fields := put["fields"].([]any)
	if len(fields) != 2 {
		t.Fatalf("duplicate fields added: %v", fields)
	}
	if f := fieldByID(put, "username"); f["value"] != "svc-user" || f["purpose"] != "USERNAME" {
		t.Errorf("built-in username not filled: %v", f)
	}
	if f := fieldByID(put, "password"); f["value"] == nil || f["value"] == "" {
		t.Errorf("built-in password not filled (generated): %v", f)
	}
}
