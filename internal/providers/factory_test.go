package providers_test

import (
	"testing"

	"github.com/elabx-org/herald/internal/providers"
)

func TestFactory_UnknownType(t *testing.T) {
	_, err := providers.Build("unknown-type-xyz-not-registered", "p", "", "", 0)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestFactory_RegisterAndBuild(t *testing.T) {
	const key = "test-factory-register-and-build"
	providers.RegisterFactory(key, func(name, url, token string, priority int) (providers.Provider, error) {
		return nil, nil
	})
	_, err := providers.Build(key, "p", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
}
