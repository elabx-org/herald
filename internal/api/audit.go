package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/elabx-org/herald/internal/audit"
)

func (s *Server) handleAuditReal(w http.ResponseWriter, r *http.Request) {
	if s.auditor == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"entries": []audit.Entry{}, "count": 0})
		return
	}

	opts := audit.QueryOptions{
		Stack:  r.URL.Query().Get("stack"),
		Secret: r.URL.Query().Get("secret"),
	}
	if h := r.URL.Query().Get("hours"); h != "" {
		opts.Hours, _ = strconv.Atoi(h)
	}

	entries, err := s.auditor.Query(opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}
