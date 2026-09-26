package safeboxstore

// CharacterSafeboxStateExporter is the first repository-style seam for the
// durable same-account safebox password + warehouse money + cell surface already
// projected onto migration 0028_character_safebox_item_instance_attributes.
// Implementations may be file-backed, hermetic in-memory, or an opt-in SQL
// repository.
//
// FileStore and MemoryStore do not open a database, emit SQL, or mutate stores
// beyond the optional Load/Save path already owned by Store. SQLStore queries
// already-owned tip-0015 + additive 0025/0028 tables through a caller-supplied
// database/sql executor; it does not select a driver, load a DSN, or register a
// production engine. Stock gamed rematerialize stays on FileStore unless a
// caller passes SQLStore into the explicit opt-in constructor. That opt-in is
// not a driver selection, DSN, upsert, auto-run, or remote-admin route.
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

// SelectRematerializeStore chooses the gamed warehouse Load/Save seam.
// Stock construction passes a nil SQLStore and keeps the FileStore.
// A non-nil SQLStore is the explicit opt-in for that one runtime: open,
// check-in, password, and money then rematerialize through the already-owned
// SQL tables. Nil fileStore with a nil opt-in stays nil. This does not select
// a driver, load a DSN, upsert, auto-run, or replace any other runtime's file.
func SelectRematerializeStore(fileStore Store, sqlStore *SQLStore) Store {
	if sqlStore != nil {
		return sqlStore
	}
	return fileStore
}
