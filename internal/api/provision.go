package api

import (
	"encoding/json"
	"net/http"

	"github.com/elabx-org/herald/internal/provisioner"
	"github.com/rs/zerolog/log"
)

type provisionRequest struct {
	Vault    string                     `json:"vault"`
	Item     string                     `json:"item"`
	Category string                     `json:"category,omitempty"` // "login", "api_credentials", "secure_note"
	Fields   map[string]provisionField  `json:"fields"`
}

type provisionField struct {
	Value     string `json:"value,omitempty"` // empty = auto-generate
	Concealed bool   `json:"concealed,omitempty"`
}

type provisionResponse struct {
	VaultID string            `json:"vault_id"`
	ItemID  string            `json:"item_id"`
	Refs    map[string]string `json:"refs"`
}

func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	var req provisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Vault == "" {
		http.Error(w, "vault is required", http.StatusBadRequest)
		return
	}
	if req.Item == "" {
		http.Error(w, "item is required", http.StatusBadRequest)
		return
	}
	if len(req.Fields) == 0 {
		http.Error(w, "at least one field is required", http.StatusBadRequest)
		return
	}

	p, err := provisioner.New()
	if err != nil {
		log.Error().Err(err).Msg("provision: failed to create provisioner")
		http.Error(w, "provisioning unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	specs := make(map[string]provisioner.FieldSpec, len(req.Fields))
	for name, f := range req.Fields {
		specs[name] = provisioner.FieldSpec{
			Value:     f.Value,
			Concealed: f.Concealed,
		}
	}

	result, err := p.Provision(r.Context(), provisioner.ProvisionRequest{
		Vault:    req.Vault,
		Item:     req.Item,
		Category: req.Category,
		Fields:   specs,
	})
	if err != nil {
		log.Error().Err(err).Str("vault", req.Vault).Str("item", req.Item).Msg("provision: failed")
		http.Error(w, "provision failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("vault", req.Vault).
		Str("item", req.Item).
		Str("item_id", result.ItemID).
		Int("fields", len(result.Refs)).
		Msg("provision: item created")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(provisionResponse{
		VaultID: result.VaultID,
		ItemID:  result.ItemID,
		Refs:    result.Refs,
	})
}
