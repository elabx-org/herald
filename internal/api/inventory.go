package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type stackInventory struct {
	Secrets       int        `json:"secrets"`
	LastSynced    *time.Time `json:"last_synced,omitempty"`
	ProvidersUsed []string   `json:"providers_used"`
	Policies      []string   `json:"policies"`
}

func (s *Server) handleInventoryReal(w http.ResponseWriter, r *http.Request) {
	stacks := s.index.All()
	inventory := make(map[string]stackInventory)

	for stack, info := range stacks {
		inv := stackInventory{
			Secrets:       info.SecretCount,
			ProvidersUsed: info.Providers,
			Policies:      info.Policies,
		}
		if !info.LastSynced.IsZero() {
			inv.LastSynced = &info.LastSynced
		}
		inventory[stack] = inv
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stacks": inventory,
	})
}
