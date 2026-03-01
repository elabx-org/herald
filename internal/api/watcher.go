package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const healthWatchInterval = 5 * time.Minute

// tokenAlertState tracks whether we've already fired an alert for a token,
// avoiding repeated alerts on every tick.
type tokenAlertState int

const (
	tokenStateOK tokenAlertState = iota
	tokenStateWarning
	tokenStateExpired
)

// StartHealthWatcher monitors provider health every 5 minutes and fires Komodo
// alerts on state transitions (ok→degraded / degraded→ok, token expiry).
// No-op if Komodo is not wired.
func (s *Server) StartHealthWatcher(ctx context.Context) {
	if s.komodo == nil {
		return
	}

	log.Info().Dur("interval", healthWatchInterval).Msg("health watcher started")

	var lastDegraded bool
	lastTokenState := make(map[string]tokenAlertState)

	check := func() {
		s.checkProviderHealth(ctx, &lastDegraded)
		s.checkTokenExpiry(ctx, lastTokenState)
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

func (s *Server) checkProviderHealth(ctx context.Context, lastDegraded *bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/v1/health", nil)
	if err != nil {
		return
	}
	resp, _ := s.getHealth(req)
	degraded := resp.Status == "degraded"

	switch {
	case degraded && !*lastDegraded:
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

	case !degraded && *lastDegraded:
		if err := s.komodo.SendAlert(ctx, "ok", "Herald: all providers healthy"); err != nil {
			log.Error().Err(err).Msg("health watcher: failed to send recovery alert")
		} else {
			log.Info().Msg("health watcher: recovery alert sent to Komodo")
		}
	}

	*lastDegraded = degraded
}

func (s *Server) checkTokenExpiry(ctx context.Context, lastState map[string]tokenAlertState) {
	if s.cfg.Alerts.TokenExpiryWarningDays == 0 {
		return
	}
	threshold := time.Duration(s.cfg.Alerts.TokenExpiryWarningDays) * 24 * time.Hour
	now := time.Now()

	for _, p := range s.cfg.Providers {
		if p.Token == "" {
			continue
		}
		expiry, err := decodeJWTExpiry(p.Token)
		if err != nil {
			continue // not a JWT or no exp claim — skip
		}

		remaining := expiry.Sub(now)
		var state tokenAlertState
		switch {
		case remaining <= 0:
			state = tokenStateExpired
		case remaining < threshold:
			state = tokenStateWarning
		default:
			state = tokenStateOK
		}

		prev := lastState[p.Name]
		lastState[p.Name] = state

		if state == prev {
			continue // no transition, no alert
		}

		switch state {
		case tokenStateExpired:
			msg := fmt.Sprintf("Herald: %s token (%s) has EXPIRED — service account access broken", p.Type, p.Name)
			if err := s.komodo.SendAlert(ctx, "critical", msg); err != nil {
				log.Error().Err(err).Str("provider", p.Name).Msg("health watcher: failed to send expiry alert")
			} else {
				log.Error().Str("provider", p.Name).Msg("health watcher: token expired alert sent")
			}
		case tokenStateWarning:
			days := int(remaining.Hours() / 24)
			msg := fmt.Sprintf("Herald: %s token (%s) expires in %d day(s) — renew before it expires", p.Type, p.Name, days)
			if err := s.komodo.SendAlert(ctx, "warning", msg); err != nil {
				log.Error().Err(err).Str("provider", p.Name).Msg("health watcher: failed to send expiry warning")
			} else {
				log.Warn().Str("provider", p.Name).Int("days", days).Msg("health watcher: token expiry warning sent")
			}
		case tokenStateOK:
			// Recovered (token was renewed) — no alert needed, just log
			log.Info().Str("provider", p.Name).Msg("health watcher: token expiry resolved")
		}
	}
}

// decodeJWTExpiry extracts the exp claim from a JWT without verifying the signature.
// Returns an error if the token is not a JWT or has no exp claim.
func decodeJWTExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decode payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("unmarshal claims: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("no exp claim")
	}
	return time.Unix(claims.Exp, 0), nil
}
