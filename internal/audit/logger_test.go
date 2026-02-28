package audit_test

import (
	"os"
	"testing"

	"github.com/elabx-org/herald/internal/audit"
)

func TestAuditLog(t *testing.T) {
	f, _ := os.CreateTemp("", "herald-audit-*.log")
	f.Close()
	defer os.Remove(f.Name())

	logger, err := audit.New(f.Name())
	if err != nil {
		t.Fatalf("audit.New() error = %v", err)
	}
	defer logger.Close()

	logger.Log(audit.Entry{
		Action:      "resolve",
		Stack:       "myapp",
		Secret:      "DB_PASSWORD",
		Provider:    "connect",
		Policy:      "memory",
		CacheHit:    false,
		DurationMs:  87,
		TriggeredBy: "herald-agent",
	})

	entries, err := logger.Query(audit.QueryOptions{Stack: "myapp"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
	if entries[0].Secret != "DB_PASSWORD" {
		t.Errorf("Secret = %q, want DB_PASSWORD", entries[0].Secret)
	}
}
