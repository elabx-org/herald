package cache_test

import (
	"os"
	"testing"
	"time"

	"github.com/elabx-org/herald/internal/cache"
)

func TestCacheSetGet(t *testing.T) {
	dir, _ := os.MkdirTemp("", "herald-cache-test-*")
	defer os.RemoveAll(dir)

	store, err := cache.New(dir+"/test.db", "test-encryption-key-32chars!!")
	if err != nil {
		t.Fatalf("cache.New() error = %v", err)
	}
	defer store.Close()

	entry := &cache.Entry{
		Value:     "super-secret",
		Provider:  "connect",
		Policy:    cache.PolicyMemory,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	key := "vault/item/field"
	if err := store.Set(key, entry); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Value != "super-secret" {
		t.Errorf("Value = %q, want super-secret", got.Value)
	}
}

func TestCacheExpiry(t *testing.T) {
	dir, _ := os.MkdirTemp("", "herald-cache-test-*")
	defer os.RemoveAll(dir)

	store, _ := cache.New(dir+"/test.db", "test-encryption-key-32chars!!")
	defer store.Close()

	store.Set("expired-key", &cache.Entry{
		Value:     "old-value",
		Policy:    cache.PolicyEncrypted,
		ExpiresAt: time.Now().Add(-1 * time.Second), // already expired
	})

	_, err := store.Get("expired-key")
	if err != cache.ErrExpired {
		t.Errorf("Get() error = %v, want ErrExpired", err)
	}
}

func TestCacheDelete(t *testing.T) {
	dir, _ := os.MkdirTemp("", "herald-cache-test-*")
	defer os.RemoveAll(dir)

	store, _ := cache.New(dir+"/test.db", "test-encryption-key-32chars!!")
	defer store.Close()

	store.Set("key", &cache.Entry{Value: "val", Policy: cache.PolicyMemory, ExpiresAt: time.Now().Add(time.Hour)})
	store.Delete("key")

	_, err := store.Get("key")
	if err != cache.ErrNotFound {
		t.Errorf("Get() after Delete() = %v, want ErrNotFound", err)
	}
}
