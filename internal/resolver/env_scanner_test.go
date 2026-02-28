package resolver_test

import (
	"strings"
	"testing"

	"github.com/elabx-org/herald/internal/resolver"
)

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
