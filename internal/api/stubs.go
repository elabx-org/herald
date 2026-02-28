package api

import (
	"encoding/json"
	"net/http"
)

// handleAudit delegates to handleAuditReal.
func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	s.handleAuditReal(w, r)
}

// handleInventory is a stub; full implementation in Task 15.
func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"stacks": map[string]interface{}{}})
}

// handleRotate is a stub; full implementation in Task 15.
func (s *Server) handleRotate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}

// handleCacheDelete is a stub; full implementation in Task 15.
func (s *Server) handleCacheDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}
