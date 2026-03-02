package providers

import "context"

type Vault struct{ ID, Name string }
type Item struct{ ID, Name, Category string }
type Field struct {
	ID       string
	Label    string
	Concealed bool
}

type Provider interface {
	Name() string
	Type() string
	Priority() int
	Resolve(ctx context.Context, vault, item, field string) (string, error)
	Healthy(ctx context.Context) (ok bool, latencyMs int64, err error)
	ListVaults(ctx context.Context) ([]Vault, error)
	ListItems(ctx context.Context, vault string) ([]Item, error)
	ListFields(ctx context.Context, vault, item string) ([]Field, error)
	Close() error
}
