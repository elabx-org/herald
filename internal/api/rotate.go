package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/elabx-org/herald/internal/audit"
	"github.com/go-chi/chi/v5"
)

type rotateResponse struct {
	ItemID          string   `json:"item_id"`
	Vault           string   `json:"vault,omitempty"`
	CacheInvalidated int     `json:"cache_invalidated"`
	StacksRedeployed []string `json:"stacks_redeployed"`
	Errors          []string `json:"errors,omitempty"`
}

func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "item")
	s.doRotate(w, r, "", itemID)
}

func (s *Server) handleRotateVault(w http.ResponseWriter, r *http.Request) {
	vault := chi.URLParam(r, "vault")
	itemID := chi.URLParam(r, "item")
	s.doRotate(w, r, vault, itemID)
}

func (s *Server) doRotate(w http.ResponseWriter, r *http.Request, vault, itemID string) {
	start := time.Now()
	resp := rotateResponse{
		ItemID: itemID,
		Vault:  vault,
	}

	// Flush matching cache entries if cache is available
	if s.opts.Manager != nil {
		count := s.opts.Manager.FlushItem(r.Context(), vault, itemID)
		resp.CacheInvalidated = count
	}

	// Fan out to integrations using context.WithoutCancel so deploys survive disconnect
	deployCtx := context.WithoutCancel(r.Context())
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, integration := range s.opts.Integrations {
		intg := integration
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := intg.Deploy(deployCtx, itemID); err != nil {
				mu.Lock()
				resp.Errors = append(resp.Errors, intg.Name()+": "+err.Error())
				mu.Unlock()
				return
			}
			mu.Lock()
			resp.StacksRedeployed = append(resp.StacksRedeployed, intg.Name())
			mu.Unlock()
		}()
	}
	wg.Wait()

	if s.opts.AuditLogger != nil {
		errMsg := ""
		if len(resp.Errors) > 0 {
			errMsg = resp.Errors[0]
		}
		s.opts.AuditLogger.Log(audit.Entry{
			Action:     "rotate",
			Secret:     itemID,
			DurationMs: time.Since(start).Milliseconds(),
			Policy:     map[bool]string{true: "ok", false: "error"}[len(resp.Errors) == 0],
			Error:      errMsg,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}
