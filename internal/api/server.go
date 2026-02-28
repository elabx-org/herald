package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elabx-org/herald/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

type Server struct {
	cfg    *config.Config
	router *chi.Mux
}

func NewServer(cfg *config.Config) *Server {
	s := &Server{cfg: cfg}
	s.router = chi.NewRouter()
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.Recoverer)
	s.mountRoutes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) mountRoutes() {
	s.router.Get("/v1/health", s.handleHealth)
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
