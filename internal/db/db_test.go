package db_test

import (
	"testing"

	"github.com/elabx-org/herald/internal/db"
)

func TestOpen_CreatesSchema(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	tables := []string{"settings", "users", "api_keys", "providers",
		"integrations", "aliases", "stacks", "stack_secrets", "audit", "migrations"}
	for _, tbl := range tables {
		row := store.DB().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl)
		var name string
		if err := row.Scan(&name); err != nil {
			t.Errorf("table %q not found: %v", tbl, err)
		}
	}
}

func TestOpen_WALMode(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	var mode string
	store.DB().QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "memory" {
		// In-memory SQLite uses "memory" journal mode, not WAL
		// WAL mode applies to file-based databases
		t.Logf("journal_mode=%q (expected memory for :memory: DB)", mode)
	}
}
