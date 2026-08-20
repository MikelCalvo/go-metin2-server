package itemstore

// ItemTemplateStateExporter is the first repository-style seam for the durable
// authored item-template surface already projected onto migration
// 0009_item_template_refine_info (after 0005 base schema and 0006 safebox-reject
// storage). Implementations may be file-backed or hermetic in-memory; none of
// these methods open a database, emit SQL, or mutate stores beyond the optional
// Load/Save path already owned by Store.
//
// Missing or empty committed snapshots are treated as an empty migration-shaped
// export.
type ItemTemplateStateExporter interface {
	ExportItemTemplateState() (ItemTemplateStateExport, error)
}

var (
	_ Store                     = (*FileStore)(nil)
	_ ItemTemplateStateExporter = (*FileStore)(nil)
	_ Store                     = (*MemoryStore)(nil)
	_ ItemTemplateStateExporter = (*MemoryStore)(nil)
)
