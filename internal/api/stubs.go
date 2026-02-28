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

// handleRotate delegates to handleRotateReal.
func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
	s.handleRotateReal(w, r)
}

// handleCacheDelete purges the cache for a specific stack.
func (s *Server) handleCacheDelete(w http.ResponseWriter, r *http.Request) {
	stack := chi.URLParam(r, "stack")
	if s.cache != nil && stack != "" {
		s.cache.DeletePrefix(stack + "/")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "stack": stack})
}
