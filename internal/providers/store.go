package providers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"

	bolt "go.etcd.io/bbolt"
)

const providersBucket = "providers"

// Record is stored in bbolt (token field is AES-GCM ciphertext).
type Record struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Priority int    `json:"priority"`
	URL      string `json:"url"`
	Token    []byte `json:"token,omitempty"` // nil = no token stored
}

// Store persists provider records to bbolt with encrypted tokens.
type Store struct {
	db  *bolt.DB
	gcm cipher.AEAD
}

// NewStore opens (or creates) the providers bucket. key must be 32 bytes (AES-256).
// Pass nil key to create a store that refuses token writes.
func NewStore(db *bolt.DB, key []byte) (*Store, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(providersBucket))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("providers bucket: %w", err)
	}
	s := &Store{db: db}
	if len(key) > 0 {
		if len(key) != 16 && len(key) != 24 && len(key) != 32 {
			return nil, fmt.Errorf("providers store: key must be 16, 24, or 32 bytes; got %d", len(key))
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		s.gcm = gcm
	}
	return s, nil
}

// Save persists rec. If token is non-empty it is encrypted; if empty the existing
// encrypted token (if any) is preserved unchanged.
func (s *Store) Save(rec Record, token string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(providersBucket))
		if b == nil {
			return fmt.Errorf("providers bucket not found")
		}

		if token != "" {
			if s.gcm == nil {
				return fmt.Errorf("cannot store token: no cache key configured (HERALD_CACHE_KEY required)")
			}
			ct, err := s.encrypt([]byte(token))
			if err != nil {
				return err
			}
			rec.Token = ct
		} else if existing := b.Get([]byte(rec.Name)); existing != nil {
			// Keep existing token
			var old Record
			if json.Unmarshal(existing, &old) == nil {
				rec.Token = old.Token
			}
		}

		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		return b.Put([]byte(rec.Name), data)
	})
}

// Delete removes a record by name.
func (s *Store) Delete(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(providersBucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(name))
	})
}

// List returns all stored records (tokens remain encrypted).
func (s *Store) List() ([]Record, error) {
	var out []Record
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(providersBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var r Record
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			out = append(out, r)
			return nil
		})
	})
	return out, err
}

// DecryptToken decrypts a stored token ciphertext. Returns "" for nil input.
func (s *Store) DecryptToken(ct []byte) (string, error) {
	if len(ct) == 0 {
		return "", nil
	}
	if s.gcm == nil {
		return "", fmt.Errorf("no key configured")
	}
	pt, err := s.decrypt(ct)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func (s *Store) encrypt(pt []byte) ([]byte, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return s.gcm.Seal(nonce, nonce, pt, nil), nil
}

func (s *Store) decrypt(ct []byte) ([]byte, error) {
	ns := s.gcm.NonceSize()
	if len(ct) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return s.gcm.Open(nil, ct[:ns], ct[ns:], nil)
}
