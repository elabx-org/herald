package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/elabx-org/herald/internal/api"
	"github.com/elabx-org/herald/internal/config"
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

	srv := api.NewServer(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("server exited with error")
	}
}
