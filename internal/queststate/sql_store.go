package queststate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// SQLStore is the first live DB-backed quest-flag repository. It implements
// Store Load/Save (and CharacterQuestStateExporter) against the already-owned
// tip-0004 character_quest_flags table through a caller-supplied database/sql
// executor. Export identity stays tip-0004. Schema preflight requires the
// ledger entry for version 4 / character_quest_state.
//
// The runtime snapshot is name-keyed and does not carry character ids. SQL
// rows are id-keyed. Load joins characters so each non-zero flag comes back
// with the roster name. Save resolves those names against the already-owned
// 0002 roster and scoped-replaces character_quest_flags for exactly the
// character ids present in the supplied snapshot. Characters absent from the
// snapshot are left untouched. An empty snapshot is a no-op: it does not
// truncate the table and it does not insert, update, or delete roster rows.
//
// The package does not select a driver, load a DSN, embed secrets, or
// register a production engine. Stock gamed rematerialize stays on FileStore
// unless a caller passes SQLStore into the explicit opt-in constructor. This
// is scoped replace of the supplied characters, not insert-only import and
// not an upsert.
type SQLStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

// NewSQLStore returns an opt-in SQL quest-flag repository. A nil executor
// fails closed on Load/Save/Export rather than at construction.
func NewSQLStore(executor dbmigrations.SQLMigrationExecutor) *SQLStore {
	return &SQLStore{executor: executor}
}

// Load projects character_quest_flags onto a name-keyed Snapshot. Empty flag
// tables are an empty snapshot, not a missing FileStore. Schema preflight
// requires tip-0004. Parent character rows supply the flag names.
func (s *SQLStore) Load() (Snapshot, error) {
	if s == nil || questStateImportExecutorIsNil(s.executor) {
		return Snapshot{}, ErrCharacterQuestStateImportExecutorRequired
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin character quest-state SQL load transaction: %w", err)
	}
	snapshot, err := loadCharacterQuestStateSnapshot(ctx, tx)
	if err != nil {
		return Snapshot{}, rollbackAfterQuestStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit character quest-state SQL load transaction: %w", err)
	}
	return snapshot, nil
}

// Save replaces character_quest_flags for every character named in snapshot,
// and also for every character id listed in the replace scope. The runtime
// snapshot is name-keyed and drops a flag whose value becomes zero, so a
// compare-and-set that clears a character's last flag passes that character id
// here to wipe those rows. Characters absent from both the snapshot and the
// scope are left untouched. An empty snapshot with an empty scope is a no-op:
// it does not truncate the table and it does not insert, update, or delete
// roster rows. Names resolve through the already-owned 0002 roster inside the
// same transaction.
func (s *SQLStore) Save(snapshot Snapshot) error {
	return s.SaveReplacing(snapshot, nil)
}

// SaveReplacing is Save plus an explicit character-id wipe scope. Ids that are
// not named in the snapshot still have their tip-0004 rows deleted. Unknown
// ids fail closed before commit. A nil or empty scope keeps Save's no-op for
// an empty snapshot.
func (s *SQLStore) SaveReplacing(snapshot Snapshot, characterIDs []uint32) error {
	if s == nil || questStateImportExecutorIsNil(s.executor) {
		return ErrCharacterQuestStateImportExecutorRequired
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate quest state snapshot", err)
	}
	scope := append([]uint32(nil), characterIDs...)
	seenScope := make(map[uint32]struct{}, len(scope))
	for _, id := range scope {
		if id == 0 {
			return fmt.Errorf("%w: quest-state replace scope contains character id 0", ErrInvalidSnapshot)
		}
		if _, exists := seenScope[id]; exists {
			return fmt.Errorf("%w: duplicate quest-state replace scope character id %d", ErrInvalidSnapshot, id)
		}
		seenScope[id] = struct{}{}
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin character quest-state SQL save transaction: %w", err)
	}
	if err := requireCharacterQuestStateSchema(ctx, tx); err != nil {
		return rollbackAfterQuestStateImportFailure(tx, err)
	}
	if err := replaceCharacterQuestStateSnapshot(ctx, tx, normalized, scope); err != nil {
		return rollbackAfterQuestStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit character quest-state SQL save transaction: %w", err)
	}
	return nil
}

// ExportCharacterQuestState projects the committed SQL flags onto the tip-0004
// export. characterIDsByName filters and names the rows the same way FileStore
// does: a flag whose roster name is absent from the map fails closed, and an
// empty flag table yields an empty export. The map is not used to invent rows.
func (s *SQLStore) ExportCharacterQuestState(characterIDsByName map[string]uint32) (CharacterQuestStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		return CharacterQuestStateExport{}, err
	}
	return ExportCharacterQuestState(snapshot, characterIDsByName)
}

// ApplyTransition applies one compare-and-set flag transition against the
// committed SQL snapshot. Characters that still have a non-zero flag are
// scoped-replaced. A transition that clears a character's last flag passes
// that roster id as an explicit wipe so the row does not survive. Other
// characters stay untouched.
func (s *SQLStore) ApplyTransition(transition Transition) (TransitionApplyResult, error) {
	if s == nil || questStateImportExecutorIsNil(s.executor) {
		return TransitionApplyResult{}, ErrCharacterQuestStateImportExecutorRequired
	}
	current, err := s.Load()
	if err != nil {
		return TransitionApplyResult{}, err
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
	wipe, err := clearedQuestFlagCharacterIDs(s, current, next)
	if err != nil {
		return TransitionApplyResult{}, err
	}
	if err := s.SaveReplacing(next, wipe); err != nil {
		return TransitionApplyResult{}, err
	}
	applyResult.Summary = summarizeSnapshot(next)
	return applyResult, nil
}

// PreviewTransition evaluates one compare-and-set flag transition without
// mutating character_quest_flags.
func (s *SQLStore) PreviewTransition(transition Transition) (TransitionApplyResult, error) {
	if s == nil || questStateImportExecutorIsNil(s.executor) {
		return TransitionApplyResult{}, ErrCharacterQuestStateImportExecutorRequired
	}
	current, err := s.Load()
	if err != nil {
		return TransitionApplyResult{}, err
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

func loadCharacterQuestStateSnapshot(ctx context.Context, tx *sql.Tx) (Snapshot, error) {
	if err := requireCharacterQuestStateSchema(ctx, tx); err != nil {
		return Snapshot{}, err
	}
	rows, err := tx.QueryContext(ctx, `
SELECT characters.name, character_quest_flags.quest_ref, character_quest_flags.flag_name, character_quest_flags.value
FROM character_quest_flags
JOIN characters ON characters.id = character_quest_flags.character_id
ORDER BY characters.name ASC, character_quest_flags.quest_ref ASC, character_quest_flags.flag_name ASC`)
	if err != nil {
		return Snapshot{}, fmt.Errorf("query character quest flags: %w", err)
	}
	defer rows.Close()

	flags := make([]Flag, 0)
	for rows.Next() {
		var (
			name     string
			questRef string
			flagName string
			value    int64
		)
		if err := rows.Scan(&name, &questRef, &flagName, &value); err != nil {
			return Snapshot{}, fmt.Errorf("scan character quest flag: %w", err)
		}
		if value <= 0 || value > int64(^uint32(0)) {
			return Snapshot{}, fmt.Errorf("%w: quest flag value %d for character %q", ErrInvalidSnapshot, value, name)
		}
		flags = append(flags, Flag{
			Character: name,
			QuestRef:  questRef,
			Name:      flagName,
			Value:     uint32(value),
		})
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("iterate character quest flags: %w", err)
	}
	snapshot := normalizeSnapshot(Snapshot{Flags: flags})
	if err := validateSnapshot(snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate loaded quest state snapshot", err)
	}
	return snapshot, nil
}

func replaceCharacterQuestStateSnapshot(ctx context.Context, tx *sql.Tx, snapshot Snapshot, characterIDs []uint32) error {
	if len(snapshot.Flags) == 0 && len(characterIDs) == 0 {
		return nil
	}
	names := make([]string, 0)
	seen := make(map[string]struct{})
	for _, flag := range snapshot.Flags {
		key := strings.ToLower(flag.Character)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, flag.Character)
	}
	ids, err := resolveQuestStateCharacterIDs(ctx, tx, names)
	if err != nil {
		return err
	}
	if err := requireQuestStateCharacterIDs(ctx, tx, characterIDs); err != nil {
		return err
	}
	replaced := make(map[uint32]struct{}, len(ids)+len(characterIDs))
	for _, id := range characterIDs {
		if _, ok := replaced[id]; ok {
			continue
		}
		if err := deleteCharacterQuestFlagsForCharacter(ctx, tx, id); err != nil {
			return err
		}
		replaced[id] = struct{}{}
	}
	for _, flag := range snapshot.Flags {
		id := ids[strings.ToLower(flag.Character)]
		if _, ok := replaced[id]; !ok {
			if err := deleteCharacterQuestFlagsForCharacter(ctx, tx, id); err != nil {
				return err
			}
			replaced[id] = struct{}{}
		}
		if err := insertCharacterQuestFlag(ctx, tx, CharacterQuestFlagRow{
			CharacterID: id,
			Character:   flag.Character,
			QuestRef:    flag.QuestRef,
			Flag:        flag.Name,
			Value:       flag.Value,
		}); err != nil {
			return err
		}
	}
	return nil
}

func requireQuestStateCharacterIDs(ctx context.Context, tx *sql.Tx, characterIDs []uint32) error {
	if len(characterIDs) == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM characters`)
	if err != nil {
		return fmt.Errorf("query quest-state character ids: %w", err)
	}
	defer rows.Close()
	known := make(map[uint32]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan quest-state character id: %w", err)
		}
		if id <= 0 || id > int64(^uint32(0)) {
			return fmt.Errorf("%w: character id %d", ErrInvalidSnapshot, id)
		}
		known[uint32(id)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate quest-state character ids: %w", err)
	}
	for _, id := range characterIDs {
		if _, ok := known[id]; !ok {
			return fmt.Errorf("%w: quest-state replace scope character id %d is not in the roster", ErrInvalidSnapshot, id)
		}
	}
	return nil
}

func clearedQuestFlagCharacterIDs(store *SQLStore, before Snapshot, after Snapshot) ([]uint32, error) {
	remaining := make(map[string]struct{}, len(after.Flags))
	for _, flag := range after.Flags {
		remaining[strings.ToLower(flag.Character)] = struct{}{}
	}
	cleared := make([]string, 0)
	seen := make(map[string]struct{})
	for _, flag := range before.Flags {
		key := strings.ToLower(flag.Character)
		if _, ok := remaining[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleared = append(cleared, flag.Character)
	}
	if len(cleared) == 0 {
		return nil, nil
	}
	ctx := context.Background()
	tx, err := store.executor.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin character quest-state wipe lookup: %w", err)
	}
	idsByName, err := resolveQuestStateCharacterIDs(ctx, tx, cleared)
	if err != nil {
		return nil, rollbackAfterQuestStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit character quest-state wipe lookup: %w", err)
	}
	ids := make([]uint32, 0, len(cleared))
	seenIDs := make(map[uint32]struct{}, len(cleared))
	for _, name := range cleared {
		id := idsByName[strings.ToLower(name)]
		if _, ok := seenIDs[id]; ok {
			continue
		}
		seenIDs[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func resolveQuestStateCharacterIDs(ctx context.Context, tx *sql.Tx, names []string) (map[string]uint32, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, name FROM characters ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query quest-state character roster: %w", err)
	}
	defer rows.Close()

	byName := make(map[string]uint32)
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan quest-state character roster: %w", err)
		}
		if id <= 0 || id > int64(^uint32(0)) {
			return nil, fmt.Errorf("%w: character id %d for %q", ErrInvalidSnapshot, id, name)
		}
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return nil, fmt.Errorf("%w: blank roster name for character id %d", ErrInvalidSnapshot, id)
		}
		if previous, exists := byName[key]; exists && previous != uint32(id) {
			return nil, fmt.Errorf("%w: character %q maps to both ids %d and %d", ErrInvalidSnapshot, name, previous, id)
		}
		byName[key] = uint32(id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quest-state character roster: %w", err)
	}

	resolved := make(map[string]uint32, len(names))
	for _, name := range names {
		key := strings.ToLower(name)
		id, ok := byName[key]
		if !ok || id == 0 {
			return nil, fmt.Errorf("%w: quest flag for unknown character %q", ErrInvalidSnapshot, name)
		}
		resolved[key] = id
	}
	return resolved, nil
}
