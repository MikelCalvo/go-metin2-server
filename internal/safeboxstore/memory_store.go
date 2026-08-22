package safeboxstore

import (
	"errors"
	"fmt"
	"sync"
)

// MemoryStore is a hermetic safebox store for tests. It implements Store without
// touching the filesystem. Backup/restore/crash-temp remain FileStore operator
// primitives.
type MemoryStore struct {
	mu        sync.RWMutex
	committed bool
	snapshot  Snapshot
}

// NewMemoryStore returns an empty hermetic safebox store with no committed
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
		return fmt.Errorf("%w: memory safebox store is nil", ErrInvalidSnapshot)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate safebox snapshot", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = cloneSnapshot(normalized)
	s.committed = true
	return nil
}

// Replace clears the committed snapshot without filesystem I/O.
func (s *MemoryStore) Replace(snapshot Snapshot) error {
	if s == nil {
		return fmt.Errorf("%w: memory safebox store is nil", ErrInvalidSnapshot)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate safebox snapshot", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = cloneSnapshot(normalized)
	s.committed = true
	return nil
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

func cloneSnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{Characters: cloneCharacterRows(snapshot.Characters)}
}

// LoadOrEmpty returns the committed snapshot or an empty one when missing.
func LoadOrEmpty(store Store) (Snapshot, error) {
	if store == nil {
		return Snapshot{Characters: []CharacterRow{}}, nil
	}
	snapshot, err := store.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return Snapshot{Characters: []CharacterRow{}}, nil
		}
		return Snapshot{}, err
	}
	return snapshot, nil
}
