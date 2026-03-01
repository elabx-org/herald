package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const healthWatchInterval = 5 * time.Minute

// StartHealthWatcher monitors provider health every 5 minutes and fires Komodo alerts
// on state transitions (ok→degraded and degraded→ok). No-op if Komodo is not wired.
func (s *Server) StartHealthWatcher(ctx context.Context) {
	if s.komodo == nil {
		return
	}

	log.Info().Dur("interval", healthWatchInterval).Msg("health watcher started")

	var lastDegraded bool

	check := func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/v1/health", nil)
		if err != nil {
			return
		}
		resp, _ := s.getHealth(req)
		degraded := resp.Status == "degraded"

		switch {
		case degraded && !lastDegraded:
			var issues []string
			for _, p := range resp.Providers {
				if p.Status != "ok" {
					detail := p.Name
					if p.Error != "" {
						detail += ": " + p.Error
					}
					issues = append(issues, detail)
				}
			}
			msg := "Herald: provider degraded"
			if len(issues) > 0 {
				msg += " — " + strings.Join(issues, "; ")
			}
			if err := s.komodo.SendAlert(ctx, "critical", msg); err != nil {
				log.Error().Err(err).Msg("health watcher: failed to send degraded alert")
			} else {
				log.Warn().Str("detail", msg).Msg("health watcher: degraded alert sent to Komodo")
			}

		case !degraded && lastDegraded:
			if err := s.komodo.SendAlert(ctx, "ok", "Herald: all providers healthy"); err != nil {
				log.Error().Err(err).Msg("health watcher: failed to send recovery alert")
			} else {
				log.Info().Msg("health watcher: recovery alert sent to Komodo")
			}
		}

		lastDegraded = degraded
	}

	ticker := time.NewTicker(healthWatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
