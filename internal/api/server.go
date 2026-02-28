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

func (s *Server) mountRoutes() {
	// Public (no auth)
	s.router.Get("/v1/health", s.handleHealth)

	// Protected routes (bearer token required when APIToken is set)
	s.router.Group(func(r chi.Router) {
		r.Use(s.bearerAuth)
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
		WriteTimeout: 15 * time.Second,
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
