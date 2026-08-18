package worldruntime

import (
	"sort"
	"sync"
)

type SessionFrameSink interface {
	Enqueue(frames [][]byte)
}

type SessionRelocator func(mapIndex uint32, x int32, y int32) (any, bool)

type SessionEntry struct {
	FrameSink SessionFrameSink
	Relocator SessionRelocator
}

type SessionDirectory struct {
	mu         sync.Mutex
	byEntityID map[uint64]SessionEntry
}

func NewSessionDirectory() *SessionDirectory {
	return &SessionDirectory{byEntityID: make(map[uint64]SessionEntry)}
}

func (d *SessionDirectory) Register(entityID uint64, entry SessionEntry) bool {
	if d == nil || entityID == 0 || !validSessionEntry(entry) {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.byEntityID[entityID]; ok {
		return false
	}
	d.byEntityID[entityID] = entry
	return true
}

func (d *SessionDirectory) Replace(entityID uint64, entry SessionEntry) bool {
	if d == nil || entityID == 0 || !validSessionEntry(entry) {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.byEntityID[entityID]; !ok {
		return false
	}
	d.byEntityID[entityID] = entry
	return true
}

func (d *SessionDirectory) Lookup(entityID uint64) (SessionEntry, bool) {
	if d == nil || entityID == 0 {
		return SessionEntry{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	entry, ok := d.byEntityID[entityID]
	return entry, ok
}

func (d *SessionDirectory) Remove(entityID uint64) (SessionEntry, bool) {
	if d == nil || entityID == 0 {
		return SessionEntry{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	entry, ok := d.byEntityID[entityID]
	if !ok {
		return SessionEntry{}, false
	}
	delete(d.byEntityID, entityID)
	return entry, true
}

// EntityIDs returns the currently registered live session entity IDs in
// ascending order. It is a read-only enumeration helper for pending-frame
// consumers such as proximity aggro acquisition.
func (d *SessionDirectory) EntityIDs() []uint64 {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.byEntityID) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(d.byEntityID))
	for entityID := range d.byEntityID {
		ids = append(ids, entityID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func validSessionEntry(entry SessionEntry) bool {
	return entry.FrameSink != nil || entry.Relocator != nil
}
