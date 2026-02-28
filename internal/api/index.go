package api

import (
	"sync"
	"time"
)

// StackInfo contains metadata about a stack's secrets.
type StackInfo struct {
	SecretCount int
	Providers   []string
	Policies    []string
	LastSynced  time.Time
	// Map of item ID -> list of env var names
	ItemRefs map[string][]string
}

// Index maintains a mapping of stacks to their secret references.
type Index struct {
	mu     sync.RWMutex
	stacks map[string]*StackInfo
}

func NewIndex() *Index {
	return &Index{stacks: make(map[string]*StackInfo)}
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

// Upsert updates or inserts stack info.
func (idx *Index) Upsert(stack string, info *StackInfo) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.stacks[stack] = info
}
