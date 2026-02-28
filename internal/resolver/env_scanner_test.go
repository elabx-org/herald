package resolver_test

import (
	"strings"
	"testing"

	"github.com/elabx-org/herald/internal/resolver"
)

func TestResolveEnvContent(t *testing.T) {
	content := "# Comment\nDB_PASSWORD=op://Vault/pg/password\nAPP_URL=https://example.com\nSMTP_KEY=op://Vault/smtp/key\n\n"
	resolved := map[string]string{
		"op://Vault/pg/password": "secret-db-pass",
		"op://Vault/smtp/key":    "secret-smtp-key",
	}

	got := resolver.ResolveEnvContent(content, resolved)

	if !strings.Contains(got, "DB_PASSWORD=secret-db-pass") {
		t.Errorf("expected DB_PASSWORD resolved, got:\n%s", got)
	}
	if !strings.Contains(got, "SMTP_KEY=secret-smtp-key") {
		t.Errorf("expected SMTP_KEY resolved, got:\n%s", got)
	}
	if !strings.Contains(got, "APP_URL=https://example.com") {
		t.Errorf("expected APP_URL preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "# Comment") {
		t.Errorf("expected comment preserved, got:\n%s", got)
	}
	if strings.Contains(got, "op://") {
		t.Errorf("expected no op:// refs in output, got:\n%s", got)
	}
}

func TestResolveEnvContentInline(t *testing.T) {
	content := "DATABASE_URL=postgresql+asyncpg://user:op://Vault/app/db_password@postgres:5432/db\nNAME=plain\n"
	resolved := map[string]string{
		"op://Vault/app/db_password": "s3cr3t",
	}

	got := resolver.ResolveEnvContent(content, resolved)

	if !strings.Contains(got, "DATABASE_URL=postgresql+asyncpg://user:s3cr3t@postgres:5432/db") {
		t.Errorf("expected inline ref resolved, got:\n%s", got)
	}
	if !strings.Contains(got, "NAME=plain") {
		t.Errorf("expected NAME preserved, got:\n%s", got)
	}
	if strings.Contains(got, "op://") {
		t.Errorf("expected no op:// in output, got:\n%s", got)
	}
}

func TestScanEnvFile(t *testing.T) {
	content := `
# Comment
DB_PASSWORD=op://Vault/postgres/password
REGULAR_VAR=not-a-secret
SMTP_KEY=op://Vault/smtp/api_key
EMPTY=
`
	refs, err := resolver.ScanEnvFile(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ScanEnvFile() error = %v", err)
	}
	if len(refs) != 2 {
		t.Errorf("got %d refs, want 2", len(refs))
	}
	if refs["op://Vault/postgres/password"].Vault != "Vault" {
		t.Errorf("vault = %q, want Vault", refs["op://Vault/postgres/password"].Vault)
	}
	if refs["op://Vault/smtp/api_key"].Field != "api_key" {
		t.Errorf("field = %q, want api_key", refs["op://Vault/smtp/api_key"].Field)
	}
}

func TestScanEnvFileInline(t *testing.T) {
	content := `DATABASE_URL=postgresql+asyncpg://user:op://Vault/app/db_password@postgres:5432/db
REGULAR=plain`
	refs, err := resolver.ScanEnvFile(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ScanEnvFile() error = %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("got %d refs, want 1", len(refs))
	}
	ref, ok := refs["op://Vault/app/db_password"]
	if !ok {
		t.Fatalf("ref op://Vault/app/db_password not found")
	}
	if ref.Vault != "Vault" || ref.Item != "app" || ref.Field != "db_password" {
		t.Errorf("ref = %+v, want Vault=Vault Item=app Field=db_password", ref)
	}
}

func TestScanEnvFileDeduplicate(t *testing.T) {
	content := `DB_PASSWORD=op://Vault/app/password
POSTGRES_PASSWORD=op://Vault/app/password
DATABASE_URL=postgresql+asyncpg://user:op://Vault/app/password@postgres:5432/db`
	refs, err := resolver.ScanEnvFile(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ScanEnvFile() error = %v", err)
	}
	// All three reference the same URI — deduplicated to 1 fetch
	if len(refs) != 1 {
		t.Errorf("got %d refs, want 1 (deduplicated)", len(refs))
	}
}
