package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type stackInventory struct {
	Secrets       int        `json:"secrets"`
	LastSynced    *time.Time `json:"last_synced,omitempty"`
	ProvidersUsed []string   `json:"providers_used"`
	Policies      []string   `json:"policies"`
}

func (s *Server) handleInventoryStackReal(w http.ResponseWriter, r *http.Request) {
	stack := chi.URLParam(r, "stack")
	info, ok := s.index.Get(stack)
	if !ok {
		http.Error(w, "stack not found", http.StatusNotFound)
		return
	}
	inv := stackInventory{
		Secrets:       info.SecretCount,
		ProvidersUsed: info.Providers,
		Policies:      info.Policies,
	}
	if !info.LastSynced.IsZero() {
		inv.LastSynced = &info.LastSynced
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inv)
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
