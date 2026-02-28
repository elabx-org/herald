package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/cache"
	"github.com/elabx-org/herald/internal/config"
	"github.com/elabx-org/herald/internal/provider"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load(os.Getenv("HERALD_CONFIG"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	mgr, err := provider.FromConfig(cfg.Providers)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create provider manager")
	}

	srv := api.NewServer(cfg, mgr)

	if cfg.Cache.EncryptionKey != "" {
		store, err := cache.New(cfg.Cache.DataPath, cfg.Cache.EncryptionKey)
		if err != nil {
			log.Fatal().Err(err).Str("path", cfg.Cache.DataPath).Msg("failed to initialize cache")
		}
		defer store.Close()
		srv.SetCache(store)
		log.Info().Str("path", cfg.Cache.DataPath).Int("ttl", cfg.Cache.DefaultTTL).Msg("cache initialized")
	} else {
		log.Warn().Msg("HERALD_CACHE_KEY not set — cache disabled, secrets fetched fresh on every request")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("server exited with error")
	}
}
