package api

import (
	"encoding/json"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"github.com/rs/zerolog/log"
)

var indexBucket = []byte("index")

// StackInfo contains metadata about a stack's secrets.
type StackInfo struct {
	SecretCount int                `json:"secret_count"`
	Providers   []string           `json:"providers"`
	Policies    []string           `json:"policies"`
	LastSynced  time.Time          `json:"last_synced"`
	ItemRefs    map[string][]string `json:"item_refs"` // item ID -> env var names
}

// Index maintains a mapping of stacks to their secret references.
// When a bbolt DB is provided via SetDB, entries survive Herald restarts.
type Index struct {
	mu     sync.RWMutex
	stacks map[string]*StackInfo
	db     *bolt.DB
}

func NewIndex() *Index {
	return &Index{stacks: make(map[string]*StackInfo)}
}

// SetDB wires a bbolt database for persistence. It creates the index bucket if
// needed and loads previously persisted entries into memory. Call once at startup.
func (idx *Index) SetDB(db *bolt.DB) {
	if db == nil {
		return
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(indexBucket)
		return err
	}); err != nil {
		log.Error().Err(err).Msg("index: failed to create bucket")
		return
	}

	var loaded int
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(indexBucket)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var info StackInfo
			if err := json.Unmarshal(v, &info); err != nil {
				log.Warn().Str("stack", string(k)).Err(err).Msg("index: skipping corrupt entry")
				return nil
			}
			idx.stacks[string(k)] = &info
			loaded++
			return nil
		})
	}); err != nil {
		log.Error().Err(err).Msg("index: failed to load persisted entries")
		return
	}

	idx.db = db
	log.Info().Int("stacks", loaded).Msg("index: loaded from persistent store")
}

// All returns a copy of the stacks map.
func (idx *Index) All() map[string]StackInfo {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	result := make(map[string]StackInfo, len(idx.stacks))
	for k, v := range idx.stacks {
		result[k] = *v
	}
	return result
}

// StacksForItem returns the list of stack names that reference the given item ID.
func (idx *Index) StacksForItem(itemID string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var stacks []string
	for name, info := range idx.stacks {
		if _, ok := info.ItemRefs[itemID]; ok {
			stacks = append(stacks, name)
		}
	}
	return stacks
}

// Delete removes a stack from the index, persisting the removal to bbolt if available.
func (idx *Index) Delete(stack string) {
	idx.mu.Lock()
	delete(idx.stacks, stack)
	db := idx.db
	idx.mu.Unlock()

	if db == nil {
		return
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(indexBucket)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(stack))
	}); err != nil {
		log.Error().Err(err).Str("stack", stack).Msg("index: failed to delete")
	}
}

// Upsert updates or inserts stack info, persisting to bbolt if available.
func (idx *Index) Upsert(stack string, info *StackInfo) {
	idx.mu.Lock()
	idx.stacks[stack] = info
	db := idx.db
	idx.mu.Unlock()

	if db == nil {
		return
	}
	data, err := json.Marshal(info)
	if err != nil {
		log.Error().Err(err).Str("stack", stack).Msg("index: failed to marshal for persistence")
		return
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(indexBucket)
		if b == nil {
			return nil
		}
		return b.Put([]byte(stack), data)
	}); err != nil {
		log.Error().Err(err).Str("stack", stack).Msg("index: failed to persist")
	}
}
