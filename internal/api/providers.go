package api

import (
	"net/http"
)

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if s.opts.Manager == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, s.opts.Manager.ProviderStatuses())
}
