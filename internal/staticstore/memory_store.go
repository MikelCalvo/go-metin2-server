package staticstore

import (
	"fmt"
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
)

// MemoryStore is a hermetic static-actor store for tests and repository-seam
// proofs. It implements Store and StaticActorContentStateExporter without
// touching the filesystem or opening a database. It deliberately omits backup,
// restore, and crash-temp cleanup: those remain FileStore operator primitives.
type MemoryStore struct {
	mu        sync.RWMutex
	committed bool
	snapshot  Snapshot
}

// NewMemoryStore returns an empty hermetic static-actor store with no committed
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
		return fmt.Errorf("%w: memory static actor store is nil", ErrInvalidSnapshot)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate static actor snapshot", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = cloneSnapshot(normalized)
	s.committed = true
	return nil
}

// ExportStaticActorContentState projects the committed in-memory snapshot onto
// the 0008 static-actor content-state migration shape without filesystem I/O.
// Missing committed snapshots on either store side are treated as empty
// collections, matching FileStore / ExportStaticActorContentStateFromStores.
func (s *MemoryStore) ExportStaticActorContentState(interactions interactionstore.Store) (StaticActorContentStateExport, error) {
	return ExportStaticActorContentStateFromStores(s, interactions)
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{
		StaticActors:   cloneStaticActors(snapshot.StaticActors),
		CombatProfiles: cloneCombatProfileSnapshotsPreservingRewardDropMultiplicity(snapshot.CombatProfiles),
	}
}
