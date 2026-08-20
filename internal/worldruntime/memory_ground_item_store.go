package worldruntime

import (
	"sync"
)

// MemoryGroundItemStore is a hermetic pending-ground-handle source for tests and
// repository-seam proofs. It implements BootstrapGroundItemStateExporter without
// touching the filesystem, opening a database, or claiming restart durability.
// It deliberately omits backup, restore, and process-restart recovery: those
// remain future operator primitives once a durable world-state repository exists.
type MemoryGroundItemStore struct {
	mu        sync.RWMutex
	snapshots []GroundItemSnapshot
}

// NewMemoryGroundItemStore returns an empty hermetic ground-item store that
// exports as an empty migration-shaped collection until Replace succeeds.
func NewMemoryGroundItemStore() *MemoryGroundItemStore {
	return &MemoryGroundItemStore{}
}

// Replace stores a deep copy of the supplied pending ground snapshots. Callers
// may pass nil to clear the store back to an empty export.
func (s *MemoryGroundItemStore) Replace(snapshots []GroundItemSnapshot) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = cloneGroundItemSnapshots(snapshots)
}

// List returns a deep copy of the committed pending ground snapshots ordered by
// ascending visible VID. An unset store returns an empty slice.
func (s *MemoryGroundItemStore) List() []GroundItemSnapshot {
	if s == nil {
		return []GroundItemSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.snapshots) == 0 {
		return []GroundItemSnapshot{}
	}
	return cloneGroundItemSnapshots(s.snapshots)
}

// ExportBootstrapGroundItemState projects the committed in-memory snapshots onto
// the 0010 bootstrap ground-item migration shape without filesystem I/O. An
// empty or unset store is treated as an empty export.
func (s *MemoryGroundItemStore) ExportBootstrapGroundItemState() (BootstrapGroundItemStateExport, error) {
	return ExportBootstrapGroundItemState(s.List())
}

// SnapshotGroundItemExporter adapts a snapshot-list callback onto
// BootstrapGroundItemStateExporter. Live shared-world readers can supply
// GroundItems without teaching the export path about registry internals.
type SnapshotGroundItemExporter struct {
	Snapshots func() []GroundItemSnapshot
}

// ExportBootstrapGroundItemState projects the adapted snapshot list onto the
// 0010 migration shape. A nil adapter or nil callback is treated as an empty
// export.
func (e SnapshotGroundItemExporter) ExportBootstrapGroundItemState() (BootstrapGroundItemStateExport, error) {
	if e.Snapshots == nil {
		return ExportBootstrapGroundItemState(nil)
	}
	return ExportBootstrapGroundItemState(e.Snapshots())
}
