package integrations

import "context"

type Event struct {
	Type  string // "rotation", "health_change"
	Stack string
	Data  map[string]string
}

type Integration interface {
	Name() string
	Type() string
	Deploy(ctx context.Context, stack string) error
	Notify(ctx context.Context, event Event) error
	Healthy(ctx context.Context) (bool, error)
}
