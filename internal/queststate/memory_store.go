package queststate

import (
	"errors"
	"fmt"
	"sync"
)

// MemoryStore is a hermetic quest-state store for tests and repository-seam
// proofs. It implements Store and CharacterQuestStateExporter without touching
// the filesystem or opening a database. It deliberately omits backup, restore,
// and crash-temp cleanup: those remain FileStore operator primitives.
//
// ApplyTransition and PreviewTransition are included so hermetic NPC/kill-quest
// gameplay tests can exercise the same compare-and-set primitive without a
// disk-backed snapshot.
type MemoryStore struct {
	mu        sync.RWMutex
	committed bool
	snapshot  Snapshot
}

// NewMemoryStore returns an empty hermetic quest-state store with no committed
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
	// Re-normalize so empty committed snapshots return Flags: []Flag{} rather
	// than a nil slice, matching FileStore Load/Save canonicalization.
	return normalizeSnapshot(s.snapshot), nil
}

func (s *MemoryStore) Save(snapshot Snapshot) error {
	if s == nil {
		return fmt.Errorf("%w: memory quest state store is nil", ErrInvalidSnapshot)
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate quest state snapshot", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = cloneSnapshot(normalized)
	s.committed = true
	return nil
}

// ExportCharacterQuestState projects the committed in-memory snapshot onto the
// 0004 character quest-state migration shape without filesystem I/O. A missing
// committed snapshot is treated as an empty export, matching FileStore.
func (s *MemoryStore) ExportCharacterQuestState(characterIDsByName map[string]uint32) (CharacterQuestStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return CharacterQuestStateExport{
				MigrationVersion: CharacterQuestStateMigrationVersion,
				MigrationName:    CharacterQuestStateMigrationName,
				Flags:            []CharacterQuestFlagRow{},
			}, nil
		}
		return CharacterQuestStateExport{}, err
	}
	return ExportCharacterQuestState(snapshot, characterIDsByName)
}

// ApplyTransition applies one compare-and-set flag transition against the
// committed in-memory snapshot. A missing committed snapshot is treated as an
// empty current snapshot for evaluation, matching FileStore.
func (s *MemoryStore) ApplyTransition(transition Transition) (TransitionApplyResult, error) {
	if s == nil {
		return TransitionApplyResult{}, fmt.Errorf("%w: memory quest state store is nil", ErrInvalidSnapshot)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	current := Snapshot{Flags: []Flag{}}
	if s.committed {
		current = cloneSnapshot(s.snapshot)
	}
	normalizedTransition := normalizeTransition(transition)
	next, result := ApplyTransition(current, normalizedTransition)
	applyResult := TransitionApplyResult{
		Transition: normalizedTransition,
		Result:     result,
	}
	if !result.Applied {
		applyResult.Summary = summarizeSnapshot(current)
		return applyResult, nil
	}
	normalizedNext := normalizeSnapshot(next)
	if err := validateSnapshot(normalizedNext); err != nil {
		return TransitionApplyResult{}, fmt.Errorf("%w: validate quest state snapshot", err)
	}
	s.snapshot = cloneSnapshot(normalizedNext)
	s.committed = true
	applyResult.Summary = summarizeSnapshot(normalizedNext)
	return applyResult, nil
}

// PreviewTransition evaluates one compare-and-set flag transition without
// mutating the committed in-memory snapshot.
func (s *MemoryStore) PreviewTransition(transition Transition) (TransitionApplyResult, error) {
	if s == nil {
		return TransitionApplyResult{}, fmt.Errorf("%w: memory quest state store is nil", ErrInvalidSnapshot)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	current := Snapshot{Flags: []Flag{}}
	if s.committed {
		current = cloneSnapshot(s.snapshot)
	}
	normalizedTransition := normalizeTransition(transition)
	next, result := ApplyTransition(current, normalizedTransition)
	applyResult := TransitionApplyResult{
		Transition: normalizedTransition,
		Result:     result,
	}
	if !result.Applied {
		applyResult.Summary = summarizeSnapshot(current)
		return applyResult, nil
	}
	applyResult.Summary = summarizeSnapshot(next)
	return applyResult, nil
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{Flags: cloneFlags(snapshot.Flags)}
}
