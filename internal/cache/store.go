package cache

import (
	"encoding/json"
	"errors"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	ErrNotFound = errors.New("cache: key not found")
	ErrExpired  = errors.New("cache: entry expired")
)

const (
	PolicyMemory    = "memory"
	PolicyTmpfs     = "tmpfs"
	PolicyEncrypted = "encrypted"
	PolicyFile      = "file"
)

var bucketName = []byte("secrets")

type Entry struct {
	Value     string    `json:"value"`
	Provider  string    `json:"provider"`
	Policy    string    `json:"policy"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Store struct {
	db  *bolt.DB
	key []byte
	mem map[string]*Entry // memory-only entries
}

func New(path, passphrase string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &Store{
		db:  db,
		key: deriveKey(passphrase),
		mem: make(map[string]*Entry),
	}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Set(cacheKey string, entry *Entry) error {
	if entry.Policy == PolicyMemory {
		s.mem[cacheKey] = entry
		return nil
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	encrypted, err := encrypt(s.key, data)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(cacheKey), encrypted)
	})
}

func (s *Store) Get(cacheKey string) (*Entry, error) {
	// Check memory cache first
	if e, ok := s.mem[cacheKey]; ok {
		if time.Now().After(e.ExpiresAt) {
			return nil, ErrExpired
		}
		return e, nil
	}

	var raw []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketName).Get([]byte(cacheKey))
		if v == nil {
			return ErrNotFound
		}
		raw = make([]byte, len(v))
		copy(raw, v)
		return nil
	})
	if err != nil {
		return nil, err
	}

	decrypted, err := decrypt(s.key, raw)
	if err != nil {
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal(decrypted, &entry); err != nil {
		return nil, err
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, ErrExpired
	}

	return &entry, nil
}

// GetStale returns an entry regardless of TTL. Used as a fallback when the
// provider is unavailable (e.g. rate limited) to serve the last-known value.
func (s *Store) GetStale(cacheKey string) (*Entry, error) {
	if e, ok := s.mem[cacheKey]; ok {
		return e, nil
	}

	var raw []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketName).Get([]byte(cacheKey))
		if v == nil {
			return ErrNotFound
		}
		raw = make([]byte, len(v))
		copy(raw, v)
		return nil
	})
	if err != nil {
		return nil, err
	}

	decrypted, err := decrypt(s.key, raw)
	if err != nil {
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal(decrypted, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (s *Store) Delete(cacheKey string) {
	delete(s.mem, cacheKey)
	s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Delete([]byte(cacheKey))
	})
}

// InvalidateByItemID invalidates all cache entries containing the given itemID
// in their key (format: vault/item/field). Returns count of invalidated entries.
func (s *Store) InvalidateByItemID(itemID string) int {
	count := 0
	// Invalidate memory entries
	for k := range s.mem {
		parts := splitCacheKey(k)
		if len(parts) >= 2 && parts[1] == itemID {
			delete(s.mem, k)
			count++
		}
	}
	// Invalidate bolt entries
	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		c := b.Cursor()
		var toDelete [][]byte
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			parts := splitCacheKey(string(k))
			if len(parts) >= 2 && parts[1] == itemID {
				toDelete = append(toDelete, append([]byte{}, k...))
				count++
			}
		}
		for _, k := range toDelete {
			b.Delete(k)
		}
		return nil
	})
	return count
}

func (s *Store) DeletePrefix(prefix string) {
	// Delete all keys with given prefix from mem
	for k := range s.mem {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(s.mem, k)
		}
	}
	// Delete from bolt
	s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		c := b.Cursor()
		for k, _ := c.Seek([]byte(prefix)); k != nil; k, _ = c.Next() {
			if len(k) < len(prefix) || string(k[:len(prefix)]) != prefix {
				break
			}
			b.Delete(k)
		}
		return nil
	})
}

func splitCacheKey(key string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			parts = append(parts, key[start:i])
			start = i + 1
		}
	}
	parts = append(parts, key[start:])
	return parts
}
