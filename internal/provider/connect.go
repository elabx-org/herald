package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ConnectProvider struct {
	name     string
	url      string
	token    string
	priority int
	client   *http.Client
}

func NewConnectProvider(name, url, token string, priority int) *ConnectProvider {
	return &ConnectProvider{
		name:     name,
		url:      url,
		token:    token,
		priority: priority,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *ConnectProvider) Name() string  { return p.name }
func (p *ConnectProvider) Priority() int { return p.priority }

func (p *ConnectProvider) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	return p.client.Do(req)
}

func (p *ConnectProvider) Healthy(ctx context.Context) (bool, int64, error) {
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults", nil)
	resp, err := p.do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()
	return resp.StatusCode == http.StatusOK, latency, nil
}

func (p *ConnectProvider) Resolve(ctx context.Context, vault, item, field string) (string, error) {
	vaultID, err := p.findVaultID(ctx, vault)
	if err != nil {
		return "", fmt.Errorf("find vault %q: %w", vault, err)
	}
	itemID, err := p.findItemID(ctx, vaultID, item)
	if err != nil {
		return "", fmt.Errorf("find item %q: %w", item, err)
	}
	return p.getField(ctx, vaultID, itemID, field)
}

func (p *ConnectProvider) findVaultID(ctx context.Context, name string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.url+"/v1/vaults", nil)
	resp, err := p.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var vaults []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vaults); err != nil {
		return "", err
	}
	for _, v := range vaults {
		if v.Name == name {
			return v.ID, nil
		}
	}
	return "", fmt.Errorf("vault %q not found", name)
}

func (p *ConnectProvider) findItemID(ctx context.Context, vaultID, title string) (string, error) {
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
		if i.Title == title {
			return i.ID, nil
		}
	}
	return "", fmt.Errorf("item %q not found in vault %q", title, vaultID)
}

func (p *ConnectProvider) getField(ctx context.Context, vaultID, itemID, fieldLabel string) (string, error) {
	url := fmt.Sprintf("%s/v1/vaults/%s/items/%s", p.url, vaultID, itemID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := p.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var item struct {
		Fields []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return "", err
	}
	for _, f := range item.Fields {
		if f.Label == fieldLabel {
			return f.Value, nil
		}
	}
	return "", fmt.Errorf("field %q not found in item %q", fieldLabel, itemID)
}
