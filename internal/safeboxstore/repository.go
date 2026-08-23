package safeboxstore

// CharacterSafeboxStateExporter is the first repository-style seam for the
// durable same-account safebox password + warehouse money + cell surface already
// projected onto migration 0015_character_safebox_money. Implementations may be
// file-backed or hermetic in-memory; none of these methods open a database, emit
// SQL, or mutate stores beyond the optional Load/Save path already owned by Store.
//
// Missing snapshots are treated as an empty migration-shaped export.
type CharacterSafeboxStateExporter interface {
	ExportCharacterSafeboxState() (CharacterSafeboxStateExport, error)
}

var (
	_ Store                         = (*FileStore)(nil)
	_ CharacterSafeboxStateExporter = (*FileStore)(nil)
	_ Store                         = (*MemoryStore)(nil)
	_ CharacterSafeboxStateExporter = (*MemoryStore)(nil)
)
