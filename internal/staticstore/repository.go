package staticstore

import "github.com/MikelCalvo/go-metin2-server/internal/interactionstore"

// StaticActorContentStateExporter is the first repository-style seam for the
// durable authored static-actor + interaction content surface already projected
// onto migration 0012_static_actor_pve_interaction_state (after the historical
// 0008_static_actor_content_state tables). Implementations may be file-backed
// or hermetic in-memory; none of these methods open a database, emit SQL, or
// mutate stores beyond the optional Load/Save path already owned by Store.
//
// The interaction store argument supplies the paired committed definitions /
// merchant catalog rows required by the migration boundary. Missing or empty
// committed snapshots on either side are treated as an empty migration-shaped
// export collection for that side.
type StaticActorContentStateExporter interface {
	ExportStaticActorContentState(interactions interactionstore.Store) (StaticActorContentStateExport, error)
}

var (
	_ Store                           = (*FileStore)(nil)
	_ StaticActorContentStateExporter = (*FileStore)(nil)
	_ Store                           = (*MemoryStore)(nil)
	_ StaticActorContentStateExporter = (*MemoryStore)(nil)
)
