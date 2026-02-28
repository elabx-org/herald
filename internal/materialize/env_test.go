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
	dir, err := os.MkdirTemp("", "herald-mat-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store, err := cache.New(dir+"/cache.db", "test-key-32chars-exactly!!!!!!")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	refs := map[string]*resolver.SecretRef{
		"op://Vault/item/password": {Vault: "Vault", Item: "item", Field: "password", Raw: "op://Vault/item/password"},
	}
	envContent := "APP_URL=https://example.com\nDB_PASSWORD=op://Vault/item/password\n"

	mat := materialize.NewEnvMaterializer(store, &mockMgr{val: "resolved-secret"}, "memory", 3600)
	outPath := dir + "/test.env"

	content, result, err := mat.Materialize(context.Background(), "myapp", refs, envContent, outPath)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if result.Resolved != 1 {
		t.Errorf("Resolved = %d, want 1", result.Resolved)
	}
	if !strings.Contains(content, "DB_PASSWORD=resolved-secret") {
		t.Errorf("content missing DB_PASSWORD=resolved-secret, got:\n%s", content)
	}
	if !strings.Contains(content, "APP_URL=https://example.com") {
		t.Errorf("content missing APP_URL pass-through, got:\n%s", content)
	}

	// File should also be written
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != content {
		t.Errorf("file content differs from returned content")
	}

	// Second call should use cache
	_, result2, err := mat.Materialize(context.Background(), "myapp", refs, envContent, "")
	if err != nil {
		t.Fatalf("second Materialize() error = %v", err)
	}
	if result2.CacheHits != 1 {
		t.Errorf("CacheHits = %d, want 1", result2.CacheHits)
	}
}

func TestMaterializeEnvNoFile(t *testing.T) {
	refs := map[string]*resolver.SecretRef{
		"op://V/i/f": {Vault: "V", Item: "i", Field: "f", Raw: "op://V/i/f"},
	}
	envContent := "SECRET=op://V/i/f\n"

	mat := materialize.NewEnvMaterializer(nil, &mockMgr{val: "val"}, "memory", 3600)

	// outPath="" means no file write
	content, result, err := mat.Materialize(context.Background(), "myapp", refs, envContent, "")
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if result.Resolved != 1 {
		t.Errorf("Resolved = %d, want 1", result.Resolved)
	}
	if !strings.Contains(content, "SECRET=val") {
		t.Errorf("content missing SECRET=val, got:\n%s", content)
	}
}

func TestMaterializeEnvInline(t *testing.T) {
	refs := map[string]*resolver.SecretRef{
		"op://Vault/item/password": {Vault: "Vault", Item: "item", Field: "password", Raw: "op://Vault/item/password"},
	}
	envContent := "DATABASE_URL=postgresql+asyncpg://user:op://Vault/item/password@db:5432/mydb\nAPP=plain\n"

	mat := materialize.NewEnvMaterializer(nil, &mockMgr{val: "s3cr3t"}, "memory", 3600)

	content, result, err := mat.Materialize(context.Background(), "myapp", refs, envContent, "")
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if result.Resolved != 1 {
		t.Errorf("Resolved = %d, want 1", result.Resolved)
	}
	if !strings.Contains(content, "DATABASE_URL=postgresql+asyncpg://user:s3cr3t@db:5432/mydb") {
		t.Errorf("inline ref not resolved, got:\n%s", content)
	}
	if strings.Contains(content, "op://") {
		t.Errorf("op:// ref not fully resolved, got:\n%s", content)
	}
	if !strings.Contains(content, "APP=plain") {
		t.Errorf("plain value not preserved, got:\n%s", content)
	}
}
