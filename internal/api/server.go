package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/elabx-org/herald/internal/audit"
	"github.com/elabx-org/herald/internal/cache"
	"github.com/elabx-org/herald/internal/config"
	"github.com/elabx-org/herald/internal/komodo"
	"github.com/elabx-org/herald/internal/provider"
	"github.com/elabx-org/herald/internal/provisioner"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

const healthCacheTTL = 60 * time.Second

type Server struct {
	cfg     *config.Config
	router  *chi.Mux
	manager *provider.Manager
	auditor *audit.Logger
	cache   *cache.Store
	komodo  *komodo.Client
	prov    *provisioner.Provisioner
	index   *Index

	healthMu        sync.RWMutex
	healthCached    *HealthResponse
	healthCheckedAt time.Time
}

func NewServer(cfg *config.Config, manager *provider.Manager) *Server {
	s := &Server{
		cfg:     cfg,
		manager: manager,
		index:   NewIndex(),
	}
	s.router = chi.NewRouter()
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Recoverer)
	s.mountRoutes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) SetAuditor(a *audit.Logger) {
	s.auditor = a
}

func (s *Server) SetCache(c *cache.Store) {
	s.cache = c
}

func (s *Server) SetKomodo(k *komodo.Client) {
	s.komodo = k
}

func (s *Server) SetProvisioner(p *provisioner.Provisioner) {
	s.prov = p
}

func (s *Server) mountRoutes() {
	// Public (no auth)
	s.router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	s.router.Get("/v1/health", s.handleHealth)

	// Protected routes (bearer token required when APIToken is set)
	s.router.Group(func(r chi.Router) {
		r.Use(s.bearerAuth)
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
				next.ServeHTTP(w, r)
			})
		})
		r.Post("/v1/materialize/env", s.handleMaterializeEnv)
		r.Post("/v1/provision", s.handleProvision)
		r.Get("/v1/audit", s.handleAudit)
		r.Get("/v1/inventory", s.handleInventory)
		r.Post("/v1/rotate/{itemID}", s.handleRotate)
		r.Delete("/v1/cache/{stack}", s.handleCacheDelete)
	})
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", addr).Msg("herald listening")
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}
