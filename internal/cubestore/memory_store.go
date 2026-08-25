package cubestore

import (
	"fmt"
	"sync"
)

// MemoryStore is a hermetic cube-recipe store for tests and runtime injection.
// It implements Store without touching the filesystem.
type MemoryStore struct {
	mu        sync.RWMutex
	committed bool
	snapshot  Snapshot
}

// NewMemoryStore returns an empty hermetic cube-recipe store with no committed
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
	return normalizeSnapshot(s.snapshot), nil
}

func (s *MemoryStore) Save(snapshot Snapshot) error {
	if s == nil {
		return fmt.Errorf("%w: memory cube recipe store is nil", ErrInvalidSnapshot)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate cube recipe snapshot", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = normalizeSnapshot(normalized)
	s.committed = true
	return nil
}

// Replace clears and commits a validated snapshot without filesystem I/O.
func (s *MemoryStore) Replace(snapshot Snapshot) error {
	return s.Save(snapshot)
}

// Clear removes the committed snapshot so Load returns ErrSnapshotNotFound.
func (s *MemoryStore) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = Snapshot{}
	s.committed = false
}

var _ Store = (*MemoryStore)(nil)
