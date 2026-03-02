package providers_test

import (
	"os"
	"testing"

	"github.com/elabx-org/herald/internal/providers"
	bolt "go.etcd.io/bbolt"
)

func TestStore_RoundTrip(t *testing.T) {
	tmp, _ := os.CreateTemp("", "providers-*.db")
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, _ := bolt.Open(tmp.Name(), 0600, nil)
	defer db.Close()

	key := make([]byte, 32) // all zeros test key
	s, err := providers.NewStore(db, key)
	if err != nil {
		t.Fatal(err)
	}

	rec := providers.Record{
		Name:     "test-connect",
		Type:     "1password-connect",
		Priority: 1,
		URL:      "http://op-connect:8080",
	}
	if err := s.Save(rec, "my-token"); err != nil {
		t.Fatal(err)
	}

	all, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 record, got %d", len(all))
	}
	if all[0].Name != "test-connect" || all[0].URL != "http://op-connect:8080" {
		t.Errorf("unexpected record: %+v", all[0])
	}

	token, err := s.DecryptToken(all[0].Token)
	if err != nil {
		t.Fatal(err)
	}
	if token != "my-token" {
		t.Errorf("expected my-token, got %s", token)
	}
}

func TestStore_Delete(t *testing.T) {
	tmp, _ := os.CreateTemp("", "providers-*.db")
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := bolt.Open(tmp.Name(), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	key := make([]byte, 32)
	s, err := providers.NewStore(db, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(providers.Record{Name: "to-delete", Type: "mock"}, ""); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete("to-delete"); err != nil {
		t.Fatal(err)
	}

	all, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 records after delete, got %d", len(all))
	}
}

func TestStore_KeepTokenOnUpdate(t *testing.T) {
	tmp, _ := os.CreateTemp("", "providers-*.db")
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := bolt.Open(tmp.Name(), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	key := make([]byte, 32)
	s, err := providers.NewStore(db, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(providers.Record{Name: "p1", Type: "mock"}, "original-token"); err != nil {
		t.Fatal(err)
	}

	// Save with empty token — should keep existing encrypted token
	if err := s.Save(providers.Record{Name: "p1", Type: "mock"}, ""); err != nil {
		t.Fatal(err)
	}

	all, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.DecryptToken(all[0].Token)
	if err != nil {
		t.Fatal(err)
	}
	if token != "original-token" {
		t.Errorf("expected original-token preserved, got %q", token)
	}
}

func TestStore_NoKeyRefusesToken(t *testing.T) {
	tmp, _ := os.CreateTemp("", "providers-*.db")
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := bolt.Open(tmp.Name(), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s, err := providers.NewStore(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Save(providers.Record{Name: "p", Type: "mock"}, "a-token")
	if err == nil {
		t.Error("expected error saving token without a key, got nil")
	}
}
