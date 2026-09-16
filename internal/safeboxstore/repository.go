package safeboxstore

// CharacterSafeboxStateExporter is the first repository-style seam for the
// durable same-account safebox password + warehouse money + cell surface already
// projected onto migration 0015_character_safebox_money. Implementations may be
// file-backed, hermetic in-memory, or an opt-in SQL repository.
//
// FileStore and MemoryStore do not open a database, emit SQL, or mutate stores
// beyond the optional Load/Save path already owned by Store. SQLStore queries
// already-owned tip-0015 tables through a caller-supplied database/sql
// executor; it does not select a driver, load a DSN, or register a production
// engine.
//
// Missing FileStore/MemoryStore snapshots are treated as an empty
// migration-shaped export. SQLStore empty tables are an empty warehouse.
type CharacterSafeboxStateExporter interface {
	ExportCharacterSafeboxState() (CharacterSafeboxStateExport, error)
}

var (
	_ Store                         = (*FileStore)(nil)
	_ CharacterSafeboxStateExporter = (*FileStore)(nil)
	_ Store                         = (*MemoryStore)(nil)
	_ CharacterSafeboxStateExporter = (*MemoryStore)(nil)
	_ Store                         = (*SQLStore)(nil)
	_ CharacterSafeboxStateExporter = (*SQLStore)(nil)
)
