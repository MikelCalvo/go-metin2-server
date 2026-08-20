package worldruntime

// BootstrapGroundItemStateExporter is the first repository-style seam for the
// pending bootstrap ground item / ground gold surface already projected onto
// migration 0010_bootstrap_ground_item_state. Implementations may be hermetic
// in-memory sources or thin adapters over live shared-world snapshot reads;
// none of these methods open a database, emit SQL, mutate live ground handles,
// or make ground state durable across process restart.
//
// Empty or missing pending sets are treated as an empty migration-shaped export.
type BootstrapGroundItemStateExporter interface {
	ExportBootstrapGroundItemState() (BootstrapGroundItemStateExport, error)
}

var (
	_ BootstrapGroundItemStateExporter = (*MemoryGroundItemStore)(nil)
	_ BootstrapGroundItemStateExporter = SnapshotGroundItemExporter{}
)
