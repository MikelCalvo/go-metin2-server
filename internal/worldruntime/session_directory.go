package worldruntime

import "sync"

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

func validSessionEntry(entry SessionEntry) bool {
	return entry.FrameSink != nil || entry.Relocator != nil
}
