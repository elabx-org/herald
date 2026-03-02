package cache

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"go.etcd.io/bbolt"
	"golang.org/x/crypto/hkdf"
)

var bucket = []byte("secrets")

type Entry struct {
	Value     string    `json:"v"`
	Provider  string    `json:"p"`
	ExpiresAt time.Time `json:"e"`
	FetchedAt time.Time `json:"f"`
}

type Store struct {
	db  *bbolt.DB
	aes cipher.AEAD
}

func Open(path, passphrase string) (*Store, error) {
	key := deriveKey(passphrase)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, aes: gcm}, nil
}

func deriveKey(passphrase string) []byte {
	h := hkdf.New(sha256.New, []byte(passphrase), []byte("herald-cache-v2"), nil)
	key := make([]byte, 32)
	io.ReadFull(h, key)
	return key
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Set(_ context.Context, key string, e Entry) error {
	e.FetchedAt = time.Now().UTC()
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	nonce := make([]byte, s.aes.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := s.aes.Seal(nonce, nonce, raw, nil)
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucket).Put([]byte(key), sealed)
	})
}

func (s *Store) get(key string) (Entry, bool, error) {
	var e Entry
	var found bool
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucket).Get([]byte(key))
		if v == nil {
			return nil
		}
		found = true
		ns := s.aes.NonceSize()
		if len(v) < ns {
			return fmt.Errorf("cache: corrupt entry for %q", key)
		}
		plain, err := s.aes.Open(nil, v[:ns], v[ns:], nil)
		if err != nil {
			return err
		}
		return json.Unmarshal(plain, &e)
	})
	return e, found, err
}

func (s *Store) Get(_ context.Context, key string) (Entry, bool, error) {
	e, found, err := s.get(key)
	if err != nil || !found {
		return e, found, err
	}
	if time.Now().After(e.ExpiresAt) {
		return Entry{}, false, nil
	}
	return e, true, nil
}

func (s *Store) GetStale(_ context.Context, key string) (Entry, bool, error) {
	return s.get(key)
}

func (s *Store) Delete(_ context.Context, key string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucket).Delete([]byte(key))
	})
}

func (s *Store) Flush() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.DeleteBucket(bucket); err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucket(bucket)
		return err
	})
}

// DB returns the underlying bbolt database for direct bucket access.
func (s *Store) DB() *bbolt.DB { return s.db }
