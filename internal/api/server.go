package api

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/elabx-org/herald/internal/core"
	"github.com/elabx-org/herald/internal/integrations"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Options struct {
	APIToken     string
	Manager      *core.Manager
	Integrations []integrations.Integration
	UIFS         fs.FS
}

type Server struct {
	router chi.Router
	opts   Options
}

func NewServer(opts Options) *Server {
	s := &Server{opts: opts, router: chi.NewRouter()}
	s.router.Use(middleware.Recoverer)
	s.router.Use(requestIDMiddleware)
	s.router.Get("/ping", s.handlePing)

	s.router.Group(func(r chi.Router) {
		r.Get("/v2/health", s.handleHealth)
		r.Get("/v1/health", s.handleHealth)
		r.Get("/v2/stats", s.handleStats)
		r.Get("/v1/stats", s.handleStats)
		r.Get("/metrics", s.handleMetrics)
	})

	s.router.Group(func(r chi.Router) {
		if opts.APIToken != "" {
			r.Use(s.bearerAuthMiddleware)
		}
		r.Use(bodySizeMiddleware(1 << 20)) // 1MB
		r.Post("/v2/materialize/env", s.handleMaterialize)
		r.Post("/v1/materialize/env", s.handleMaterialize)
		r.Get("/v2/inventory", s.handleInventory)
		r.Get("/v1/inventory", s.handleInventory)
		r.Get("/v2/inventory/{stack}", s.handleInventoryStack)
		r.Get("/v1/inventory/{stack}", s.handleInventoryStack)
		r.Get("/v2/audit", s.handleAudit)
		r.Get("/v1/audit", s.handleAudit)
		r.Post("/v2/rotate/{item}", s.handleRotate)
		r.Post("/v1/rotate/{itemID}", s.handleRotate)
		r.Post("/v2/rotate/{vault}/{item}", s.handleRotateVault)
		r.Post("/v1/rotate/{vault}/{itemID}", s.handleRotateVault)
		r.Delete("/v2/cache/{stack}", s.handleCacheDeleteStack)
		r.Delete("/v1/cache/{stack}", s.handleCacheDeleteStack)
		r.Delete("/v2/cache", s.handleCacheFlush)
		r.Delete("/v1/cache", s.handleCacheFlush)
		r.Post("/v2/provision", s.handleProvision)
		r.Post("/v1/provision", s.handleProvision)
		r.Get("/v2/events", s.handleSSE)
	})

	// Serve embedded UI (when built with embed_ui tag)
	if opts.UIFS != nil {
		s.router.Handle("/*", http.FileServer(http.FS(opts.UIFS)))
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]int{})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("# Prometheus metrics\n"))
}

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleInventoryStack(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleCacheDeleteStack(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleCacheFlush(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}


func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, errCode, msg, requestID string) {
	writeJSON(w, code, map[string]string{
		"error":      errCode,
		"message":    msg,
		"request_id": requestID,
	})
}
