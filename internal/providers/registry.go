package providers

import "sort"

type Registry struct {
	providers []Provider
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Register(p Provider) {
	r.providers = append(r.providers, p)
}

func (r *Registry) Ordered() []Provider {
	sorted := make([]Provider, len(r.providers))
	copy(sorted, r.providers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})
	return sorted
}
