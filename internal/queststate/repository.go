package queststate

// CharacterQuestStateExporter is the first repository-style seam for the
// durable character quest-flag surface already projected onto migration
// 0004_character_quest_state. Implementations may be file-backed, hermetic
// in-memory, or an opt-in SQL repository.
//
// FileStore and MemoryStore do not open a database, emit SQL, or mutate stores
// beyond the optional Load/Save path already owned by Store. SQLStore queries
// already-owned tip-0004 character_quest_flags through a caller-supplied
// database/sql executor; it does not select a driver, load a DSN, or register a
// production engine. Stock gamed rematerialize stays on FileStore. Passing
// SQLStore to SelectRematerializeStore is the caller opt-in; it is not a
// driver selection, DSN, upsert, auto-run, or remote-admin route.
//
// characterIDsByName may use original or normalized character-name keys; lookups
// are case-insensitive. Missing FileStore/MemoryStore snapshots are treated as
// an empty export. SQLStore empty tables are an empty snapshot.
type CharacterQuestStateExporter interface {
	ExportCharacterQuestState(characterIDsByName map[string]uint32) (CharacterQuestStateExport, error)
}

var (
	_ Store                       = (*FileStore)(nil)
	_ CharacterQuestStateExporter = (*FileStore)(nil)
	_ Store                       = (*MemoryStore)(nil)
	_ CharacterQuestStateExporter = (*MemoryStore)(nil)
	_ Store                       = (*SQLStore)(nil)
	_ CharacterQuestStateExporter = (*SQLStore)(nil)
)

// SelectRematerializeStore chooses the quest-flag Load/Save seam.
// A nil SQLStore keeps the FileStore. A non-nil SQLStore is the explicit
// opt-in for that caller. Nil fileStore with a nil opt-in stays nil. This
// does not select a driver, load a DSN, upsert, auto-run, or mount gamed.
func SelectRematerializeStore(fileStore Store, sqlStore *SQLStore) Store {
	if sqlStore != nil {
		return sqlStore
	}
	return fileStore
}
