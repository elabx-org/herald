package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/elabx-org/herald/internal/audit"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type rotateResponse struct {
	ItemID           string   `json:"item_id"`
	CacheInvalidated int      `json:"cache_invalidated"`
	StacksRedeployed []string `json:"stacks_redeployed"`
}

func (s *Server) handleRotateReal(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "itemID")
	s.doRotate(w, r, "", itemID)
}

func (s *Server) handleRotateVaultItemReal(w http.ResponseWriter, r *http.Request) {
	vault := chi.URLParam(r, "vault")
	itemID := chi.URLParam(r, "itemID")
	s.doRotate(w, r, vault, itemID)
}

func (s *Server) doRotate(w http.ResponseWriter, r *http.Request, vault, itemID string) {
	// Invalidate cache entries for this item
	invalidated := 0
	if s.cache != nil {
		if vault != "" {
			invalidated = s.cache.InvalidateByVaultAndItemID(vault, itemID)
		} else {
			invalidated = s.cache.InvalidateByItemID(itemID)
		}
	}

	// Detach from the HTTP request context so that Komodo deploys are not cancelled
	// if the caller disconnects before all stacks finish redeploying.
	deployCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Minute)
	defer cancel()

	// Find stacks that reference this item and redeploy
	var redeployed []string
	if s.komodo != nil {
		var stacks []string
		if vault != "" {
			stacks = s.index.StacksForVaultAndItem(vault, itemID)
		} else {
			stacks = s.index.StacksForItem(itemID)
		}
		for _, stack := range stacks {
			if err := s.komodo.DeployStack(deployCtx, stack); err != nil {
				log.Error().Err(err).Str("stack", stack).Msg("failed to redeploy after rotation")
				continue
			}
			redeployed = append(redeployed, stack)
			if s.auditor != nil {
				s.auditor.Log(audit.Entry{
					Action:      "rotate",
					Stack:       stack,
					Secret:      itemID,
					TriggeredBy: "rotation-webhook",
					Timestamp:   time.Now().UTC(),
				})
			}
		}
	}

	if redeployed == nil {
		redeployed = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rotateResponse{
		ItemID:           itemID,
		CacheInvalidated: invalidated,
		StacksRedeployed: redeployed,
	})
}
