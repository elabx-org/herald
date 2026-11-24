package provisioner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ConnectProvisioner creates items in 1Password using the Connect server REST API.
type ConnectProvisioner struct {
	url    string
	token  string
	client *http.Client
}

// NewConnectProvisioner creates a provisioner backed by the Connect server.
func NewConnectProvisioner(url, token string) *ConnectProvisioner {
	return &ConnectProvisioner{
		url:    strings.TrimRight(url, "/"),
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *ConnectProvisioner) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	return p.client.Do(req)
}

// connectError includes Connect's own explanation (its JSON "message", or the start of the
// body), which is what tells you *why* a request was rejected.
func connectError(op string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	var e struct {
		Message string `json:"message"`
	}
	msg := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		msg = e.Message
	}
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if msg == "" {
		return fmt.Errorf("connect %s: HTTP %d", op, resp.StatusCode)
	}
	return fmt.Errorf("connect %s: HTTP %d: %s", op, resp.StatusCode, msg)
}

type connectVaultEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type connectItemEntry struct {
	ID       string              `json:"id,omitempty"`
	Vault    connectVaultRef     `json:"vault"`
	Title    string              `json:"title"`
	Category string              `json:"category"`
	Fields   []connectFieldEntry `json:"fields,omitempty"`
}

type connectVaultRef struct {
	ID string `json:"id"`
}

type connectFieldEntry struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Type    string `json:"type"` // "STRING" or "CONCEALED"
	Purpose string `json:"purpose,omitempty"`
	Value   string `json:"value"`
}

// builtinPurpose maps a requested field to the item category's built-in field: a LOGIN item
// has its own username and password fields (purpose USERNAME / PASSWORD). Writing them as
// plain fields instead left the built-ins empty next to same-named custom fields, so
// op://…/username resolved to the empty built-in.
func builtinPurpose(category, name string) string {
	if connectCategory(category) != "LOGIN" {
		return ""
	}
	switch strings.ToLower(name) {
	case "username":
		return "USERNAME"
	case "password":
		return "PASSWORD"
	}
	return ""
}

func (p *ConnectProvisioner) Provision(ctx context.Context, req ProvisionRequest) (ProvisionResult, error) {
	vaultID, err := p.findVaultID(ctx, req.Vault)
	if err != nil {
		return ProvisionResult{}, err
	}

	existingID, err := p.findItemID(ctx, vaultID, req.Item)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("check existing item: %w", err)
	}

	if existingID != "" {
		return p.upsert(ctx, vaultID, existingID, req)
	}
	return p.create(ctx, vaultID, req)
}

func (p *ConnectProvisioner) findVaultID(ctx context.Context, name string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults", nil)
	resp, err := p.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var vaults []connectVaultEntry
	if err := json.NewDecoder(resp.Body).Decode(&vaults); err != nil {
		return "", err
	}
	for _, v := range vaults {
		if strings.EqualFold(v.Name, name) {
			return v.ID, nil
		}
	}
	return "", fmt.Errorf("vault %q not found", name)
}

func (p *ConnectProvisioner) findItemID(ctx context.Context, vaultID, title string) (string, error) {
	url := fmt.Sprintf("%s/v1/vaults/%s/items", p.url, vaultID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := p.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var items []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return "", err
	}
	for _, i := range items {
		if strings.EqualFold(i.Title, title) {
			return i.ID, nil
		}
	}
	return "", nil
}

func (p *ConnectProvisioner) create(ctx context.Context, vaultID string, req ProvisionRequest) (ProvisionResult, error) {
	var fields []connectFieldEntry
	for name, spec := range req.Fields {
		value := spec.Value
		if value == "" {
			var err error
			value, err = generatePassword(24)
			if err != nil {
				return ProvisionResult{}, fmt.Errorf("generate password for field %q: %w", name, err)
			}
		}
		fieldType := "STRING"
		purpose := builtinPurpose(req.Category, name)
		if spec.Concealed || isLikelySecret(name) || purpose == "PASSWORD" {
			fieldType = "CONCEALED"
		}
		if purpose == "USERNAME" {
			fieldType = "STRING"
		}
		fields = append(fields, connectFieldEntry{ID: strings.ToLower(name), Label: strings.ToLower(name), Type: fieldType, Purpose: purpose, Value: value})
	}

	item := connectItemEntry{
		Vault:    connectVaultRef{ID: vaultID},
		Title:    req.Item,
		Category: connectCategory(req.Category),
		Fields:   fields,
	}
	body, _ := json.Marshal(item)

	url := fmt.Sprintf("%s/v1/vaults/%s/items", p.url, vaultID)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	resp, err := p.do(httpReq)
	if err != nil {
		return ProvisionResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return ProvisionResult{}, connectError("create item", resp)
	}

	var created connectItemEntry
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return ProvisionResult{}, err
	}
	refs := make(map[string]string, len(created.Fields))
	for _, f := range created.Fields {
		if f.ID != "" {
			refs[f.ID] = fmt.Sprintf("op://%s/%s/%s", req.Vault, req.Item, f.ID)
		}
	}
	return ProvisionResult{VaultID: vaultID, ItemID: created.ID, Refs: refs}, nil
}

// upsert adds missing fields to an existing item (and replaces existing ones only when a
// field asks to Overwrite). Connect's PUT replaces the whole item, so the item is kept as raw
// JSON: its version, sections, URLs, tags and every field's purpose and section survive.
// Decoding into a reduced struct and writing that back made Connect reject the update (400).
func (p *ConnectProvisioner) upsert(ctx context.Context, vaultID, itemID string, req ProvisionRequest) (ProvisionResult, error) {
	url := fmt.Sprintf("%s/v1/vaults/%s/items/%s", p.url, vaultID, itemID)
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := p.do(getReq)
	if err != nil {
		return ProvisionResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ProvisionResult{}, connectError("get item", resp)
	}

	var item map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return ProvisionResult{}, err
	}
	fields, _ := item["fields"].([]any)
	index := make(map[string]map[string]any, len(fields))
	for _, f := range fields {
		if m, ok := f.(map[string]any); ok {
			if id, _ := m["id"].(string); id != "" {
				if _, taken := index[id]; !taken {
					index[id] = m
				}
			}
		}
	}
	// A login's built-in username/password fields win over same-named custom fields
	category, _ := item["category"].(string)
	for _, f := range fields {
		if m, ok := f.(map[string]any); ok {
			switch m["purpose"] {
			case "USERNAME":
				index["username"] = m
			case "PASSWORD":
				index["password"] = m
			}
		}
	}

	changed := false
	for name, spec := range req.Fields {
		key := name
		if builtinPurpose(category, name) != "" {
			key = strings.ToLower(name)
		}
		if existing, ok := index[key]; ok {
			current, _ := existing["value"].(string)
			switch {
			case current == "":
				// An empty field (e.g. a login's untouched built-in) is filled, not skipped
				value := spec.Value
				if value == "" {
					if value, err = generatePassword(24); err != nil {
						return ProvisionResult{}, fmt.Errorf("generate password for field %q: %w", name, err)
					}
				}
				existing["value"] = value
				changed = true
			case spec.Overwrite && spec.Value != "" && current != spec.Value:
				existing["value"] = spec.Value
				changed = true
			}
			continue
		}
		value := spec.Value
		if value == "" {
			value, err = generatePassword(24)
			if err != nil {
				return ProvisionResult{}, fmt.Errorf("generate password for field %q: %w", name, err)
			}
		}
		fieldType := "STRING"
		if spec.Concealed || isLikelySecret(name) {
			fieldType = "CONCEALED"
		}
		fields = append(fields, map[string]any{"id": name, "label": name, "type": fieldType, "value": value})
		changed = true
	}

	if changed {
		item["fields"] = fields
		body, _ := json.Marshal(item)
		putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
		putResp, err := p.do(putReq)
		if err != nil {
			return ProvisionResult{}, err
		}
		defer putResp.Body.Close()
		if putResp.StatusCode != http.StatusOK {
			return ProvisionResult{}, connectError("update item", putResp)
		}
		item = map[string]any{}
		if err := json.NewDecoder(putResp.Body).Decode(&item); err != nil {
			return ProvisionResult{}, err
		}
		fields, _ = item["fields"].([]any)
	}

	refs := make(map[string]string, len(fields))
	for _, f := range fields {
		if m, ok := f.(map[string]any); ok {
			if id, _ := m["id"].(string); id != "" {
				refs[id] = fmt.Sprintf("op://%s/%s/%s", req.Vault, req.Item, id)
			}
		}
	}
	itemIDOut, _ := item["id"].(string)
	if itemIDOut == "" {
		itemIDOut = itemID
	}
	return ProvisionResult{VaultID: vaultID, ItemID: itemIDOut, Refs: refs}, nil
}

func connectCategory(cat string) string {
	switch strings.ToLower(cat) {
	case "api_credentials", "api-credentials", "apicredentials":
		return "API_CREDENTIAL"
	case "secure_note", "secure-note", "securenote", "note":
		return "SECURE_NOTE"
	default:
		return "LOGIN"
	}
}
