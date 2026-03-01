package provisioner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

type connectVaultEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type connectItemEntry struct {
	ID       string            `json:"id,omitempty"`
	Vault    connectVaultRef   `json:"vault"`
	Title    string            `json:"title"`
	Category string            `json:"category"`
	Fields   []connectFieldEntry `json:"fields,omitempty"`
}

type connectVaultRef struct {
	ID string `json:"id"`
}

type connectFieldEntry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // "STRING" or "CONCEALED"
	Value string `json:"value"`
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
		if spec.Concealed || isLikelySecret(name) {
			fieldType = "CONCEALED"
		}
		fields = append(fields, connectFieldEntry{ID: name, Label: name, Type: fieldType, Value: value})
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
		return ProvisionResult{}, fmt.Errorf("connect create item: HTTP %d", resp.StatusCode)
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

func (p *ConnectProvisioner) upsert(ctx context.Context, vaultID, itemID string, req ProvisionRequest) (ProvisionResult, error) {
	url := fmt.Sprintf("%s/v1/vaults/%s/items/%s", p.url, vaultID, itemID)
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := p.do(getReq)
	if err != nil {
		return ProvisionResult{}, err
	}
	defer resp.Body.Close()

	var existing connectItemEntry
	if err := json.NewDecoder(resp.Body).Decode(&existing); err != nil {
		return ProvisionResult{}, err
	}

	existingIDs := make(map[string]struct{}, len(existing.Fields))
	for _, f := range existing.Fields {
		existingIDs[f.ID] = struct{}{}
	}

	addedAny := false
	for name, spec := range req.Fields {
		if _, ok := existingIDs[name]; ok {
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
		existing.Fields = append(existing.Fields, connectFieldEntry{ID: name, Label: name, Type: fieldType, Value: value})
		addedAny = true
	}

	if addedAny {
		body, _ := json.Marshal(existing)
		putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
		putResp, err := p.do(putReq)
		if err != nil {
			return ProvisionResult{}, err
		}
		defer putResp.Body.Close()
		if putResp.StatusCode != http.StatusOK {
			return ProvisionResult{}, fmt.Errorf("connect update item: HTTP %d", putResp.StatusCode)
		}
		if err := json.NewDecoder(putResp.Body).Decode(&existing); err != nil {
			return ProvisionResult{}, err
		}
	}

	refs := make(map[string]string, len(existing.Fields))
	for _, f := range existing.Fields {
		if f.ID != "" {
			refs[f.ID] = fmt.Sprintf("op://%s/%s/%s", req.Vault, req.Item, f.ID)
		}
	}
	return ProvisionResult{VaultID: vaultID, ItemID: existing.ID, Refs: refs}, nil
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
