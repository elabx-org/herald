//go:build !onepassword_sdk

package main

import "github.com/elabx-org/herald/internal/providers"

func registerSDKProvider(ps []providers.Provider) []providers.Provider {
	return ps
}
