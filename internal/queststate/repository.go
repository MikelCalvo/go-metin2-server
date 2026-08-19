package queststate

// CharacterQuestStateExporter is the first repository-style seam for the
// durable character quest-flag surface already projected onto migration
// 0004_character_quest_state. Implementations may be file-backed or hermetic
// in-memory; none of these methods open a database, emit SQL, or mutate stores
// beyond the optional Load/Save path already owned by Store.
//
// characterIDsByName may use original or normalized character-name keys; lookups
// are case-insensitive. Missing snapshots are treated as an empty export.
type CharacterQuestStateExporter interface {
	ExportCharacterQuestState(characterIDsByName map[string]uint32) (CharacterQuestStateExport, error)
}

var (
	_ Store                       = (*FileStore)(nil)
	_ CharacterQuestStateExporter = (*FileStore)(nil)
	_ Store                       = (*MemoryStore)(nil)
	_ CharacterQuestStateExporter = (*MemoryStore)(nil)
)
