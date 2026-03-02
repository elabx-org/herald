//go:build onepassword_sdk

package onepassword

import (
	"context"
	"fmt"
	"strings"
	"time"

	onepasswordsdk "github.com/1password/onepassword-sdk-go"
	"github.com/elabx-org/herald/internal/providers"
)

type SDKProvider struct {
	name     string
	priority int
	client   *onepasswordsdk.Client
}

func NewSDK(name, token string, priority int) (*SDKProvider, error) {
	client, err := onepasswordsdk.NewClient(
		context.Background(),
		onepasswordsdk.WithServiceAccountToken(token),
		onepasswordsdk.WithIntegrationInfo("Herald v2", "2.0.0"),
	)
	if err != nil {
		return nil, fmt.Errorf("sdk provider: failed to create client: %w", err)
	}
	return &SDKProvider{name: name, priority: priority, client: client}, nil
}

func (p *SDKProvider) Name() string  { return p.name }
func (p *SDKProvider) Type() string  { return "service_account" }
func (p *SDKProvider) Priority() int { return p.priority }
func (p *SDKProvider) Close() error  { return nil }

func (p *SDKProvider) Resolve(ctx context.Context, vault, item, field string) (string, error) {
	ref := fmt.Sprintf("op://%s/%s/%s", vault, item, field)
	val, err := p.client.Secrets().Resolve(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("sdk: resolve %q: %w", ref, err)
	}
	return val, nil
}

func (p *SDKProvider) Healthy(ctx context.Context) (bool, int64, error) {
	start := time.Now()
	_, err := p.client.Vaults().List(ctx)
	if err != nil {
		return false, 0, err
	}
	return true, time.Since(start).Milliseconds(), nil
}

func (p *SDKProvider) ListVaults(ctx context.Context) ([]providers.Vault, error) {
	vaultList, err := p.client.Vaults().List(ctx)
	if err != nil {
		return nil, err
	}
	var vaults []providers.Vault
	for _, v := range vaultList {
		vaults = append(vaults, providers.Vault{ID: v.ID, Name: v.Title})
	}
	return vaults, nil
}

func (p *SDKProvider) ListItems(ctx context.Context, vault string) ([]providers.Item, error) {
	// Resolve vault name → ID
	vaultID, err := p.resolveVaultID(ctx, vault)
	if err != nil {
		return nil, err
	}
	itemList, err := p.client.Items().List(ctx, vaultID)
	if err != nil {
		return nil, err
	}
	var items []providers.Item
	for _, i := range itemList {
		items = append(items, providers.Item{ID: i.ID, Name: i.Title})
	}
	return items, nil
}

func (p *SDKProvider) ListFields(ctx context.Context, vault, item string) ([]providers.Field, error) {
	vaultID, err := p.resolveVaultID(ctx, vault)
	if err != nil {
		return nil, err
	}
	itemID, err := p.resolveItemID(ctx, vaultID, item)
	if err != nil {
		return nil, err
	}
	it, err := p.client.Items().Get(ctx, vaultID, itemID)
	if err != nil {
		return nil, err
	}
	var fields []providers.Field
	for _, f := range it.Fields {
		fields = append(fields, providers.Field{
			ID:        f.ID,
			Label:     f.Title,
			Concealed: f.FieldType == onepasswordsdk.ItemFieldTypeConcealed,
		})
	}
	return fields, nil
}

func (p *SDKProvider) resolveVaultID(ctx context.Context, vaultName string) (string, error) {
	vaults, err := p.client.Vaults().List(ctx)
	if err != nil {
		return "", err
	}
	for _, v := range vaults {
		if strings.EqualFold(v.Title, vaultName) {
			return v.ID, nil
		}
	}
	return "", fmt.Errorf("sdk: vault %q not found", vaultName)
}

func (p *SDKProvider) resolveItemID(ctx context.Context, vaultID, itemTitle string) (string, error) {
	items, err := p.client.Items().List(ctx, vaultID)
	if err != nil {
		return "", err
	}
	for _, i := range items {
		if strings.EqualFold(i.Title, itemTitle) {
			return i.ID, nil
		}
	}
	return "", fmt.Errorf("sdk: item %q not found", itemTitle)
}
