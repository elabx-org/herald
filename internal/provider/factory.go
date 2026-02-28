package provider

import (
	"fmt"
	"sort"

	"github.com/elabx-org/herald/internal/config"
)

// FromConfig builds a Manager from the config's provider list.
func FromConfig(providers []config.ProviderConfig) (*Manager, error) {
	var ps []Provider
	for _, pc := range providers {
		switch pc.Type {
		case "connect_server":
			ps = append(ps, NewConnectProvider(pc.Name, pc.URL, pc.Token, pc.Priority))
		case "service_account":
			p, err := NewServiceAccountProvider(pc.Name, pc.Token, pc.Priority)
			if err != nil {
				return nil, fmt.Errorf("provider %q: %w", pc.Name, err)
			}
			ps = append(ps, p)
		default:
			return nil, fmt.Errorf("unknown provider type: %q", pc.Type)
		}
	}
	sort.Slice(ps, func(i, j int) bool {
		return ps[i].Priority() < ps[j].Priority()
	})
	return NewManager(ps), nil
}
