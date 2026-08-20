package interactionstore

import (
	"fmt"
	"sync"
)

// MemoryStore is a hermetic interaction-definition store for tests and
// repository-seam proofs. It implements Store without touching the filesystem
// or opening a database. It deliberately omits backup, restore, and crash-temp
// cleanup: those remain FileStore operator primitives.
type MemoryStore struct {
	mu        sync.RWMutex
	committed bool
	snapshot  Snapshot
}

// NewMemoryStore returns an empty hermetic interaction store with no committed
// snapshot (Load returns ErrSnapshotNotFound until the first successful Save).
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Load() (Snapshot, error) {
	if s == nil {
		return Snapshot{}, ErrSnapshotNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.committed {
		return Snapshot{}, ErrSnapshotNotFound
	}
	return cloneSnapshot(s.snapshot), nil
}

func (s *MemoryStore) Save(snapshot Snapshot) error {
	if s == nil {
		return fmt.Errorf("%w: memory interaction store is nil", ErrInvalidSnapshot)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate interaction snapshot", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = cloneSnapshot(normalized)
	s.committed = true
	return nil
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{Definitions: cloneDefinitions(snapshot.Definitions)}
}

var _ Store = (*MemoryStore)(nil)
