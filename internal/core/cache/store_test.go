package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/elabx-org/herald/internal/core/cache"
)

func TestCache_SetAndGet(t *testing.T) {
	s, err := cache.Open(t.TempDir()+"/cache.db", "test-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	entry := cache.Entry{
		Value:     "supersecret",
		Provider:  "mock",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := s.Set(context.Background(), "HomeLab/myapp/db_password", entry); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.Get(context.Background(), "HomeLab/myapp/db_password")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected to find entry")
	}
	if got.Value != "supersecret" {
		t.Errorf("got %q, want supersecret", got.Value)
	}
}

func TestCache_StaleGet(t *testing.T) {
	s, err := cache.Open(t.TempDir()+"/cache.db", "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	expired := cache.Entry{
		Value:     "old-secret",
		Provider:  "mock",
		ExpiresAt: time.Now().Add(-time.Second),
	}
	s.Set(context.Background(), "V/I/F", expired)

	_, found, _ := s.Get(context.Background(), "V/I/F")
	if found {
		t.Error("Get should not return expired entry")
	}

	stale, found, _ := s.GetStale(context.Background(), "V/I/F")
	if !found {
		t.Error("GetStale should return expired entry")
	}
	if stale.Value != "old-secret" {
		t.Errorf("unexpected stale value: %q", stale.Value)
	}
}

func TestCache_Flush(t *testing.T) {
	s, err := cache.Open(t.TempDir()+"/cache.db", "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.Set(context.Background(), "V/I/F", cache.Entry{Value: "x", ExpiresAt: time.Now().Add(time.Hour)})
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	_, found, _ := s.Get(context.Background(), "V/I/F")
	if found {
		t.Error("entry should be gone after flush")
	}
}
