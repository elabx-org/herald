package resolver_test

import (
	"strings"
	"testing"

	"github.com/elabx-org/herald/internal/resolver"
)

func TestResolveEnvContent(t *testing.T) {
	content := "# Comment\nDB_PASSWORD=op://Vault/pg/password\nAPP_URL=https://example.com\nSMTP_KEY=op://Vault/smtp/key\n\n"
	resolved := map[string]string{
		"DB_PASSWORD": "secret-db-pass",
		"SMTP_KEY":    "secret-smtp-key",
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
	if refs["DB_PASSWORD"].Vault != "Vault" {
		t.Errorf("DB_PASSWORD vault = %q, want Vault", refs["DB_PASSWORD"].Vault)
	}
	if refs["SMTP_KEY"].Field != "api_key" {
		t.Errorf("SMTP_KEY field = %q, want api_key", refs["SMTP_KEY"].Field)
	}
}
