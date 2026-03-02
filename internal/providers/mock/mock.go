package mock

import (
	"context"
	"fmt"
	"os"

	"github.com/elabx-org/herald/internal/providers"
	"gopkg.in/yaml.v3"
)

type secretsFile struct {
	Secrets map[string]map[string]map[string]string `yaml:"secrets"`
}

type Provider struct {
	name     string
	path     string
	priority int
	data     secretsFile
}

func New(name, path string, priority int) (*Provider, error) {
	p := &Provider{name: name, path: path, priority: priority}
	return p, p.reload()
}

func (p *Provider) reload() error {
	raw, err := os.ReadFile(p.path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(raw, &p.data)
}

func (p *Provider) Name() string  { return p.name }
func (p *Provider) Type() string  { return "mock" }
func (p *Provider) Priority() int { return p.priority }
func (p *Provider) Close() error  { return nil }

func (p *Provider) Resolve(_ context.Context, vault, item, field string) (string, error) {
	v, ok := p.data.Secrets[vault]
	if !ok {
		return "", fmt.Errorf("mock: vault %q not found", vault)
	}
	i, ok := v[item]
	if !ok {
		return "", fmt.Errorf("mock: item %q not found in vault %q", item, vault)
	}
	f, ok := i[field]
	if !ok {
		return "", fmt.Errorf("mock: field %q not found in %q/%q", field, vault, item)
	}
	return f, nil
}

func (p *Provider) Healthy(_ context.Context) (bool, int64, error) { return true, 0, nil }

func (p *Provider) ListVaults(_ context.Context) ([]providers.Vault, error) {
	var vaults []providers.Vault
	for name := range p.data.Secrets {
		vaults = append(vaults, providers.Vault{ID: name, Name: name})
	}
	return vaults, nil
}

func (p *Provider) ListItems(_ context.Context, vault string) ([]providers.Item, error) {
	var items []providers.Item
	for name := range p.data.Secrets[vault] {
		items = append(items, providers.Item{ID: name, Name: name})
	}
	return items, nil
}

func (p *Provider) ListFields(_ context.Context, vault, item string) ([]providers.Field, error) {
	var fields []providers.Field
	for name := range p.data.Secrets[vault][item] {
		fields = append(fields, providers.Field{ID: name, Label: name})
	}
	return fields, nil
}
