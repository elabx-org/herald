package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Startup validation — surface common misconfigurations early
	if len(cfg.Providers) == 0 {
		log.Warn().Msg("no secret providers configured — all materialize calls will fail")
	}
	for _, p := range cfg.Providers {
		if p.Token == "" {
			log.Warn().Str("provider", p.Name).Str("type", p.Type).Msg("provider has no token — will fail to resolve secrets")
		}
	}
	if cfg.Komodo.URL != "" && (cfg.Komodo.APIKey == "" || cfg.Komodo.APISecret == "") {
		log.Warn().Msg("KOMODO_URL set but KOMODO_API_KEY or KOMODO_API_SECRET missing — rotation redeployment disabled")
	}
	if cfg.Audit.Enabled && cfg.Audit.Path == "" {
		log.Warn().Msg("audit enabled but HERALD_AUDIT_PATH not set — audit logging disabled")
	}
	if cfg.APIToken == "" {
		log.Warn().Msg("HERALD_API_TOKEN not set — API is unauthenticated")
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
	var auditor *audit.Logger
	if cfg.Audit.Enabled && cfg.Audit.Path != "" {
		var err error
		auditor, err = audit.New(cfg.Audit.Path)
		if err != nil {
			log.Fatal().Err(err).Str("path", cfg.Audit.Path).Msg("failed to initialize auditor")
		}
		defer auditor.Close()
		srv.SetAuditor(auditor)
		log.Info().Str("path", cfg.Audit.Path).Int("retention_days", cfg.Audit.RetentionDays).Msg("auditor initialized")
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

	// Start background goroutines (ctx now in scope)
	if auditor != nil && cfg.Audit.RetentionDays > 0 {
		go func() {
			if err := auditor.Prune(cfg.Audit.RetentionDays); err != nil {
				log.Warn().Err(err).Msg("audit: initial prune failed")
			}
			t := time.NewTicker(24 * time.Hour)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if err := auditor.Prune(cfg.Audit.RetentionDays); err != nil {
						log.Warn().Err(err).Msg("audit: daily prune failed")
					}
				}
			}
		}()
	}

	go srv.StartHealthWatcher(ctx)

	if err := srv.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("server exited with error")
	}
}
