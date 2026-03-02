package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/elabx-org/herald/internal/audit"
	"github.com/elabx-org/herald/internal/core"
	"github.com/elabx-org/herald/internal/providers"
)

type providerRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Priority int    `json:"priority"`
	URL      string `json:"url"`
	Token    string `json:"token"` // empty = keep existing
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if s.opts.Manager == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, s.opts.Manager.ProviderStatuses())
}

func (s *Server) handleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var req providerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Type == "" {
		http.Error(w, "name and type are required", http.StatusBadRequest)
		return
	}
	if req.Token != "" && s.opts.ProviderStore == nil {
		http.Error(w, "token storage requires HERALD_CACHE_KEY", http.StatusBadRequest)
		return
	}
	if s.opts.ProviderStore == nil {
		http.Error(w, "provider store not available", http.StatusServiceUnavailable)
		return
	}

	rec := providers.Record{
		Name: req.Name, Type: req.Type, Priority: req.Priority, URL: req.URL,
	}
	if err := s.opts.ProviderStore.Save(rec, req.Token); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Re-fetch to get final encrypted token
	all, _ := s.opts.ProviderStore.List()
	var savedToken string
	for _, u := range all {
		if u.Name == req.Name {
			savedToken, _ = s.opts.ProviderStore.DecryptToken(u.Token)
			break
		}
	}

	p, err := providers.Build(req.Type, req.Name, req.URL, savedToken, req.Priority)
	if err != nil {
		_ = s.opts.ProviderStore.Delete(req.Name) // rollback
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.opts.Manager.AddProvider(p, core.ProviderMeta{URL: req.URL, Source: "db"}); err != nil {
		_ = s.opts.ProviderStore.Delete(req.Name)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if s.opts.AuditLogger != nil {
		s.opts.AuditLogger.Log(audit.Entry{Action: "provider.create", Stack: req.Name})
	}

	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name, "source": "db"})
}

func (s *Server) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req providerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if s.opts.ProviderStore == nil {
		http.Error(w, "provider store not available", http.StatusServiceUnavailable)
		return
	}

	rec := providers.Record{
		Name: name, Type: req.Type, Priority: req.Priority, URL: req.URL,
	}
	// Save (preserves existing token if req.Token is empty)
	if err := s.opts.ProviderStore.Save(rec, req.Token); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Re-fetch updated record to get final token
	all, _ := s.opts.ProviderStore.List()
	var finalToken string
	for _, u := range all {
		if u.Name == name {
			finalToken, _ = s.opts.ProviderStore.DecryptToken(u.Token)
			break
		}
	}

	p, err := providers.Build(req.Type, name, req.URL, finalToken, req.Priority)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.opts.Manager.UpdateProvider(p, core.ProviderMeta{URL: req.URL, Source: "db"}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.opts.AuditLogger != nil {
		s.opts.AuditLogger.Log(audit.Entry{Action: "provider.update", Stack: name})
	}

	writeJSON(w, http.StatusOK, map[string]string{"name": name, "source": "db"})
}

func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if s.opts.ProviderStore == nil {
		http.Error(w, "provider store not available", http.StatusServiceUnavailable)
		return
	}
	if s.opts.Manager == nil {
		http.Error(w, "provider manager not available", http.StatusServiceUnavailable)
		return
	}

	// Find the provider status to check source
	var found *core.ProviderStatus
	for _, st := range s.opts.Manager.ProviderStatuses() {
		if st.Name == name {
			cp := st
			found = &cp
			break
		}
	}
	if found == nil {
		http.Error(w, "provider not found", http.StatusNotFound)
		return
	}

	// Check if DB record exists
	all, _ := s.opts.ProviderStore.List()
	hasDBRecord := false
	for _, rec := range all {
		if rec.Name == name {
			hasDBRecord = true
			break
		}
	}

	if found.Source == "env" && !hasDBRecord {
		http.Error(w, "cannot delete env-managed provider", http.StatusForbidden)
		return
	}

	if hasDBRecord {
		if err := s.opts.ProviderStore.Delete(name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// For env providers with a DB override: deleting the DB record reverts to env config on restart.
	// The in-memory provider continues with the overridden config until the server restarts.
	// This is intentional — env providers are managed by infrastructure and always exist at startup.

	// If db source, fully remove. If env with db override, we deleted the override above.
	if found.Source == "db" {
		_ = s.opts.Manager.RemoveProvider(name)
	}

	if s.opts.AuditLogger != nil {
		s.opts.AuditLogger.Log(audit.Entry{Action: "provider.delete", Stack: name})
	}

	w.WriteHeader(http.StatusNoContent)
}
