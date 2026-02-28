package api

import (
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

	// Invalidate cache entries for this item
	invalidated := 0
	if s.cache != nil {
		invalidated = s.cache.InvalidateByItemID(itemID)
	}

	// Find stacks that reference this item and redeploy
	var redeployed []string
	if s.komodo != nil {
		stacks := s.index.StacksForItem(itemID)
		for _, stack := range stacks {
			if err := s.komodo.DeployStack(r.Context(), stack); err != nil {
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
