package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	onepassword "github.com/1password/onepassword-sdk-go"
	"github.com/rs/zerolog/log"
)

type ServiceAccountProvider struct {
	name     string
	token    string
	priority int
	client   *onepassword.Client

	rateMu        sync.Mutex
	rateLimitedAt *time.Time
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
	_, err := p.client.Vaults().List(ctx)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		if strings.Contains(err.Error(), "rate limit") {
			p.rateMu.Lock()
			if p.rateLimitedAt == nil {
				t := start
				p.rateLimitedAt = &t
				log.Warn().
					Str("provider", p.name).
					Str("rate_limited_since", t.Format(time.RFC3339)).
					Msg("1Password rate limit detected — provider degraded")
			}
			since := *p.rateLimitedAt
			p.rateMu.Unlock()
			return false, latency, fmt.Errorf("rate limited since %s", since.Format(time.RFC3339))
		}
		p.rateMu.Lock()
		p.rateLimitedAt = nil
		p.rateMu.Unlock()
		return false, latency, err
	}
	p.rateMu.Lock()
	if p.rateLimitedAt != nil {
		log.Info().Str("provider", p.name).Msg("1Password rate limit cleared — provider healthy")
		p.rateLimitedAt = nil
	}
	p.rateMu.Unlock()
	return true, latency, nil
}

// RateLimitedSince returns when rate limiting was first detected, or nil if not currently rate limited.
func (p *ServiceAccountProvider) RateLimitedSince() *time.Time {
	p.rateMu.Lock()
	defer p.rateMu.Unlock()
	if p.rateLimitedAt == nil {
		return nil
	}
	t := *p.rateLimitedAt
	return &t
}
