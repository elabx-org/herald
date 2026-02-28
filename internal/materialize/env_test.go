package materialize_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/elabx-org/herald/internal/cache"
	"github.com/elabx-org/herald/internal/materialize"
	"github.com/elabx-org/herald/internal/resolver"
)

type mockMgr struct{ val string }

func (m *mockMgr) Resolve(ctx context.Context, vault, item, field string) (string, string, error) {
	return m.val, "mock", nil
}

func TestMaterializeEnv(t *testing.T) {
	dir, _ := os.MkdirTemp("", "herald-mat-*")
	defer os.RemoveAll(dir)

	store, _ := cache.New(dir+"/cache.db", "test-key-32chars-exactly!!!!!!")
	defer store.Close()

	refs := map[string]*resolver.SecretRef{
		"DB_PASSWORD": {Vault: "Vault", Item: "item", Field: "password", Raw: "op://Vault/item/password"},
	}

	mat := materialize.NewEnvMaterializer(store, &mockMgr{val: "resolved-secret"}, "memory", 3600)
	outPath := dir + "/test.env"

	result, err := mat.Materialize(context.Background(), "myapp", refs, outPath)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if result.Resolved != 1 {
		t.Errorf("Resolved = %d, want 1", result.Resolved)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "DB_PASSWORD=resolved-secret") {
		t.Errorf("env file missing DB_PASSWORD=resolved-secret, got:\n%s", data)
	}
}
