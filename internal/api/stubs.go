package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleAudit delegates to handleAuditReal.
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	s.handleAuditReal(w, r)
}

// handleInventory delegates to handleInventoryReal.
func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	s.handleInventoryReal(w, r)
}

// handleInventoryStack delegates to handleInventoryStackReal.
func (s *Server) handleInventoryStack(w http.ResponseWriter, r *http.Request) {
	s.handleInventoryStackReal(w, r)
}

// handleRotate delegates to handleRotateReal.
func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
	s.handleRotateReal(w, r)
}

// handleRotateVaultItem delegates to handleRotateVaultItemReal.
func (s *Server) handleRotateVaultItem(w http.ResponseWriter, r *http.Request) {
	s.handleRotateVaultItemReal(w, r)
}

// handleCacheDelete purges cache entries for a specific stack and removes it from the index.
// Cache keys are vault/item/field — we derive the exact keys from the stack's item refs.
func (s *Server) handleCacheDelete(w http.ResponseWriter, r *http.Request) {
	stack := chi.URLParam(r, "stack")
	deleted := 0
	if s.cache != nil && stack != "" {
		if info, ok := s.index.Get(stack); ok {
			for _, uris := range info.ItemRefs {
				for _, uri := range uris {
					// uri is op://vault/item/field; cache key is vault/item/field
					if len(uri) > 5 {
						s.cache.Delete(uri[5:]) // strip "op://"
						deleted++
					}
				}
			}
		}
	}
	s.index.Delete(stack)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "stack": stack, "entries_deleted": deleted})
}

// handleCacheFlush purges the entire cache.
func (s *Server) handleCacheFlush(w http.ResponseWriter, r *http.Request) {
	if s.cache != nil {
		s.cache.Flush()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}
