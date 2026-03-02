package onepassword

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elabx-org/herald/internal/providers"
)

type ConnectProvider struct {
	name     string
	url      string
	token    string
	priority int
	client   *http.Client
}

func NewConnect(name, url, token string, priority int) (*ConnectProvider, error) {
	return &ConnectProvider{
		name:     name,
		url:      url,
		token:    token,
		priority: priority,
		client:   &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (p *ConnectProvider) Name() string  { return p.name }
func (p *ConnectProvider) Type() string  { return "connect_server" }
func (p *ConnectProvider) Priority() int { return p.priority }
func (p *ConnectProvider) Close() error  { return nil }

func (p *ConnectProvider) Resolve(ctx context.Context, vault, item, field string) (string, error) {
	vaultID, err := p.resolveVaultID(ctx, vault)
	if err != nil {
		return "", err
	}
	itemObj, err := p.resolveItem(ctx, vaultID, item)
	if err != nil {
		return "", err
	}
	for _, f := range itemObj.Fields {
		if f.Label == field {
			return f.Value, nil
		}
	}
	return "", fmt.Errorf("connect: field %q not found in %s/%s", field, vault, item)
}

func (p *ConnectProvider) Healthy(ctx context.Context) (bool, int64, error) {
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults", nil)
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return false, 0, err
	}
	resp.Body.Close()
	return resp.StatusCode == 200, time.Since(start).Milliseconds(), nil
}

func (p *ConnectProvider) ListVaults(ctx context.Context) ([]providers.Vault, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults", nil)
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw []struct{ ID, Name string }
	json.NewDecoder(resp.Body).Decode(&raw)
	var vaults []providers.Vault
	for _, v := range raw {
		vaults = append(vaults, providers.Vault{ID: v.ID, Name: v.Name})
	}
	return vaults, nil
}

func (p *ConnectProvider) ListItems(ctx context.Context, vault string) ([]providers.Item, error) {
	vaultID, err := p.resolveVaultID(ctx, vault)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults/"+vaultID+"/items", nil)
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw []struct {
		ID       string
		Title    string
		Category string
	}
	json.NewDecoder(resp.Body).Decode(&raw)
	var items []providers.Item
	for _, i := range raw {
		items = append(items, providers.Item{ID: i.ID, Name: i.Title, Category: i.Category})
	}
	return items, nil
}

func (p *ConnectProvider) ListFields(ctx context.Context, vault, item string) ([]providers.Field, error) {
	vaultID, _ := p.resolveVaultID(ctx, vault)
	obj, err := p.resolveItem(ctx, vaultID, item)
	if err != nil {
		return nil, err
	}
	var fields []providers.Field
	for _, f := range obj.Fields {
		fields = append(fields, providers.Field{ID: f.ID, Label: f.Label, Concealed: f.Type == "CONCEALED"})
	}
	return fields, nil
}

type connectItem struct {
	ID     string
	Fields []struct {
		ID    string
		Label string
		Value string
		Type  string
	}
}

func (p *ConnectProvider) resolveVaultID(ctx context.Context, name string) (string, error) {
	vaults, err := p.ListVaults(ctx)
	if err != nil {
		return "", err
	}
	for _, v := range vaults {
		if strings.EqualFold(v.Name, name) {
			return v.ID, nil
		}
	}
	return "", fmt.Errorf("connect: vault %q not found", name)
}

func (p *ConnectProvider) resolveItem(ctx context.Context, vaultID, itemName string) (connectItem, error) {
	// Fetch all items and find by title (avoids URL-encoding issues with filter queries)
	url := fmt.Sprintf("%s/v1/vaults/%s/items", p.url, vaultID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return connectItem{}, err
	}
	defer resp.Body.Close()
	var items []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	json.NewDecoder(resp.Body).Decode(&items)
	var itemID string
	for _, i := range items {
		if strings.EqualFold(i.Title, itemName) {
			itemID = i.ID
			break
		}
	}
	if itemID == "" {
		return connectItem{}, fmt.Errorf("connect: item %q not found in vault", itemName)
	}
	// Fetch full item with fields
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/vaults/%s/items/%s", p.url, vaultID, itemID), nil)
	req2.Header.Set("Authorization", "Bearer "+p.token)
	resp2, err := p.client.Do(req2)
	if err != nil {
		return connectItem{}, err
	}
	defer resp2.Body.Close()
	var full connectItem
	json.NewDecoder(resp2.Body).Decode(&full)
	return full, nil
}
