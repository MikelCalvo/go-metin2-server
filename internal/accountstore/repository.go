package accountstore

// AccountLister is the narrow read seam used by migration-shaped exporters and
// operator preflight. FileStore and MemoryStore both satisfy it. Backup,
// restore, and crash-temp cleanup remain FileStore-specific and are
// intentionally not part of this interface.
type AccountLister interface {
	List() ([]Account, error)
}

// AccountCharacterStateExporter is the first repository-style seam for the
// durable PvE character surfaces already projected onto migrations
// 0002_account_character_roster, 0003_character_item_state,
// 0011_character_point_state, and 0023_character_myshop_unit_prices.
// FileStore and MemoryStore satisfy the full surface without opening a
// database. SQLItemStateStore satisfies only ExportCharacterItemState
// against already-owned tip-0003 + additive 0024/0027 tables; it does not
// claim roster, point, or myshop SQL ownership and does not select a driver.
type AccountCharacterStateExporter interface {
	ExportAccountCharacterRoster() (AccountCharacterRosterExport, error)
	ExportCharacterItemState() (CharacterItemStateExport, error)
	ExportCharacterPointState() (CharacterPointStateExport, error)
	ExportCharacterMyShopUnitPrices() (CharacterMyShopUnitPricesExport, error)
}

var (
	_ Store                         = (*FileStore)(nil)
	_ AccountLister                 = (*FileStore)(nil)
	_ AccountCharacterStateExporter = (*FileStore)(nil)
	_ Store                         = (*MemoryStore)(nil)
	_ AccountLister                 = (*MemoryStore)(nil)
	_ AccountCharacterStateExporter = (*MemoryStore)(nil)
	_ Store                         = (*SQLItemStateStore)(nil)
)
