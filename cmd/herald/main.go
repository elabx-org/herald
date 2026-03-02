package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/config"
	"github.com/elabx-org/herald/internal/core"
	"github.com/elabx-org/herald/internal/core/cache"
	"github.com/elabx-org/herald/internal/providers"
	mockprovider "github.com/elabx-org/herald/internal/providers/mock"
	opprovider "github.com/elabx-org/herald/internal/providers/onepassword"
	"github.com/elabx-org/herald/internal/integrations"
	komodoint "github.com/elabx-org/herald/internal/integrations/komodo"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfgPath := os.Getenv("HERALD_CONFIG")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Cache
	var cacheStore *cache.Store
	if cfg.Cache.Key != "" {
		cacheStore, err = cache.Open(cfg.Cache.DataPath, cfg.Cache.Key)
		if err != nil {
			log.Fatal().Err(err).Str("path", cfg.Cache.DataPath).Msg("failed to open cache")
		}
		defer cacheStore.Close()
		log.Info().Str("path", cfg.Cache.DataPath).Msg("cache initialized")
	} else {
		log.Warn().Msg("HERALD_CACHE_KEY not set — cache disabled")
	}

	// Providers
	var ps []providers.Provider

	if mockPath := os.Getenv("HERALD_MOCK_SECRETS"); mockPath != "" {
		mp, err := mockprovider.New("mock", mockPath, 99)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load mock provider")
		}
		ps = append(ps, mp)
		log.Info().Str("path", mockPath).Msg("mock provider initialized")
	}

	if url := os.Getenv("OP_CONNECT_SERVER_URL"); url != "" {
		if token := os.Getenv("OP_CONNECT_TOKEN"); token != "" {
			cp, err := opprovider.NewConnect("1password-connect", url, token, 0)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to initialize Connect provider")
			}
			ps = append(ps, cp)
			log.Info().Str("url", url).Msg("1Password Connect provider initialized")
		}
	}

	ps = registerSDKProvider(ps)

	var mgr *core.Manager
	if len(ps) > 0 && cacheStore != nil {
		mgr = core.NewManager(cacheStore, ps, time.Duration(cfg.Cache.DefaultTTLSecs)*time.Second)
	} else if len(ps) > 0 {
		// No cache — create a no-op store won't work; log warning
		log.Warn().Msg("providers configured but cache disabled — secrets fetched on every request (no caching)")
	}

	// Integrations
	var integrationList []integrations.Integration
	if cfg.Komodo.URL != "" && cfg.Komodo.APIKey != "" && cfg.Komodo.APISecret != "" {
		ki := komodoint.New("komodo", cfg.Komodo.URL, cfg.Komodo.APIKey, cfg.Komodo.APISecret)
		integrationList = append(integrationList, ki)
		log.Info().Str("url", cfg.Komodo.URL).Msg("Komodo integration initialized")
	}

	// Server
	srv := api.NewServer(api.Options{
		APIToken:     os.Getenv("HERALD_API_TOKEN"),
		Manager:      mgr,
		Integrations: integrationList,
	})

	httpSrv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		log.Info().Int("port", cfg.Port).Msg("herald starting")
		if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutCtx)
	log.Info().Msg("shutdown complete")
}
