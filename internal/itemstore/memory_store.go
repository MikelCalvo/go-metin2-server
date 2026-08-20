package itemstore

import (
	"errors"
	"fmt"
	"sync"
)

// MemoryStore is a hermetic item-template store for tests and repository-seam
// proofs. It implements Store and ItemTemplateStateExporter without touching the
// filesystem or opening a database. It deliberately omits backup, restore, and
// crash-temp cleanup: those remain FileStore operator primitives.
type MemoryStore struct {
	mu        sync.RWMutex
	committed bool
	snapshot  Snapshot
}

// NewMemoryStore returns an empty hermetic item-template store with no committed
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
	return Snapshot{Templates: cloneTemplates(s.snapshot.Templates)}, nil
}

func (s *MemoryStore) Save(snapshot Snapshot) error {
	if s == nil {
		return fmt.Errorf("%w: memory item template store is nil", ErrInvalidSnapshot)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate item template snapshot", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = Snapshot{Templates: cloneTemplates(normalized.Templates)}
	s.committed = true
	return nil
}

// ExportItemTemplateState projects the committed in-memory snapshot onto the
// 0009 item-template refine-info migration shape without filesystem I/O. A
// missing committed snapshot is treated as an empty export, matching FileStore.
func (s *MemoryStore) ExportItemTemplateState() (ItemTemplateStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return ExportItemTemplateState(Snapshot{})
		}
		return ItemTemplateStateExport{}, err
	}
	return ExportItemTemplateState(snapshot)
}
