//go:build onepassword_sdk

package main

import (
	"os"

	"github.com/rs/zerolog/log"

	"github.com/elabx-org/herald/internal/providers"
	opprovider "github.com/elabx-org/herald/internal/providers/onepassword"
)

func init() {
	providers.RegisterFactory("1password-sdk", func(name, url, token string, priority int) (providers.Provider, error) {
		// url is not used by the SDK provider; token is the service account token
		return opprovider.NewSDK(name, token, priority)
	})
}

func registerSDKProvider(ps []providers.Provider) []providers.Provider {
	if token := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN"); token != "" {
		p, err := opprovider.NewSDK("1password-sdk", token, 1)
		if err != nil {
			log.Error().Err(err).Msg("failed to initialize 1Password SDK provider")
			return ps
		}
		ps = append(ps, p)
		log.Info().Msg("1Password SDK provider initialized")
	}
	return ps
}
