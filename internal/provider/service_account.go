package provider

import (
	"context"
	"fmt"
	"time"

	onepassword "github.com/1password/onepassword-sdk-go"
)

type ServiceAccountProvider struct {
	name     string
	token    string
	priority int
	client   *onepassword.Client
}

func NewServiceAccountProvider(name, token string, priority int) (*ServiceAccountProvider, error) {
	if token == "" {
		return nil, fmt.Errorf("service account token is required")
	}
	client, err := onepassword.NewClient(
		context.Background(),
		onepassword.WithServiceAccountToken(token),
		onepassword.WithIntegrationInfo("herald", "1.0.0"),
	)
	if err != nil {
		return nil, fmt.Errorf("create 1password client: %w", err)
	}
	return &ServiceAccountProvider{
		name:     name,
		token:    token,
		priority: priority,
		client:   client,
	}, nil
}

func (p *ServiceAccountProvider) Name() string  { return p.name }
func (p *ServiceAccountProvider) Priority() int { return p.priority }

func (p *ServiceAccountProvider) Resolve(ctx context.Context, vault, item, field string) (string, error) {
	// op:// URI format: op://vault/item/field
	secretRef := fmt.Sprintf("op://%s/%s/%s", vault, item, field)
	val, err := p.client.Secrets().Resolve(ctx, secretRef)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", secretRef, err)
	}
	return val, nil
}

func (p *ServiceAccountProvider) Healthy(ctx context.Context) (bool, int64, error) {
	start := time.Now()
	// Try listing vaults as a health check
	_, err := p.client.Vaults().List(ctx)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return false, latency, err
	}
	return true, latency, nil
}
