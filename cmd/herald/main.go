package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/audit"
	"github.com/elabx-org/herald/internal/cache"
	"github.com/elabx-org/herald/internal/config"
	"github.com/elabx-org/herald/internal/komodo"
	"github.com/elabx-org/herald/internal/provider"
	"github.com/elabx-org/herald/internal/provisioner"
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

	// Wire auditor
	if cfg.Audit.Enabled && cfg.Audit.Path != "" {
		auditor, err := audit.New(cfg.Audit.Path)
		if err != nil {
			log.Fatal().Err(err).Str("path", cfg.Audit.Path).Msg("failed to initialize auditor")
		}
		defer auditor.Close()
		srv.SetAuditor(auditor)
		log.Info().Str("path", cfg.Audit.Path).Msg("auditor initialized")
	}

	// Wire Komodo client
	if cfg.Komodo.URL != "" && cfg.Komodo.APIKey != "" && cfg.Komodo.APISecret != "" {
		srv.SetKomodo(komodo.NewClient(cfg.Komodo.URL, cfg.Komodo.APIKey, cfg.Komodo.APISecret))
		log.Info().Str("url", cfg.Komodo.URL).Msg("komodo client initialized")
	}

	// Wire provisioner — prefer Connect (no rate limits, Write access)
	if url := os.Getenv("OP_CONNECT_SERVER_URL"); url != "" {
		if token := os.Getenv("OP_CONNECT_TOKEN"); token != "" {
			srv.SetProvisioner(provisioner.NewConnectProvisioner(url, token))
			log.Info().Str("url", url).Msg("connect provisioner initialized")
		} else {
			log.Warn().Msg("OP_CONNECT_SERVER_URL set but OP_CONNECT_TOKEN missing — /v1/provision unavailable")
		}
	} else if p, err := provisioner.New(); err == nil {
		srv.SetProvisioner(p)
		log.Info().Msg("sdk provisioner initialized")
	} else {
		log.Warn().Err(err).Msg("OP_PROVISION_TOKEN not set — /v1/provision unavailable")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("server exited with error")
	}
}
