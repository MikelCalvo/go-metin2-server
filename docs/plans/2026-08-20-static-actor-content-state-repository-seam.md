# Static-Actor Content-State Repository Seam — 2026-08-20

## Objective

Land the fifth narrow repository-style seam for durable authored static-actor + interaction content already projected onto migration `0008_static_actor_content_state`, so export and future backfill tooling can target a named interface instead of anonymous `FileStore` / free-function coupling, and hermetic tests can prove reduced bootstrap filesystem coupling without claiming a DB-backed runtime.

## Contract frozen by this slice

1. `staticstore.StaticActorContentStateExporter` exposes:
   - `ExportStaticActorContentState(interactions interactionstore.Store) (StaticActorContentStateExport, error)`
2. `FileStore` continues to satisfy `Store` and `StaticActorContentStateExporter` with no disk-path behavior change; the new method delegates to the existing `ExportStaticActorContentStateFromStores` helper.
3. New hermetic `staticstore.MemoryStore` also satisfies those two interfaces from an in-memory snapshot:
   - no filesystem writes
   - no SQL / DSN
   - deep-copies static actors (including spawn home, combat HP/ready-at, reward drops) and combat profiles on Load/Save
   - missing committed snapshot returns `ErrSnapshotNotFound` from `Load` and an empty migration-shaped export when paired with a missing/empty interaction store
   - deliberately omits backup / restore / crash-temp cleanup
4. New hermetic `interactionstore.MemoryStore` satisfies `Store` from an in-memory snapshot with the same hermetic guarantees (deep-copy definitions/catalogs; no backup/restore/crash-temp).
5. `gameRuntime.ExportStaticActorContentState` asserts the named `StaticActorContentStateExporter` interface on the static-actor store instead of always calling the free-function helper with anonymous `Store` values.
6. Existing quarantine validators remain the fail-closed preflight for retained export artifacts. This slice does **not** add import/backfill into live stores or databases.

## What this is not yet

- SQL-backed repository implementation
- DB connection pool / driver selection for runtime stores
- INSERT / backfill / restore-from-export tooling
- broadening `Store` into backup/restore/crash-temp
- ground-item restart durability
- remote admin API

## TDD and validation

Focused coverage:

- `go test ./internal/staticstore -run 'MemoryStore|StaticActorContentStateExporter' -count=1`
- `go test ./internal/interactionstore -run 'MemoryStore' -count=1`
- `go test ./internal/minimal -run 'ExportsStaticActorContentStateThroughMemoryStoreSeam|StaticActorContentStateExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
2. ~~Extract the bootstrap ground-handle repository seam once quarantine + live export both prove the `0010` boundary.~~ Done: `BootstrapGroundItemStateExporter` + hermetic `MemoryGroundItemStore` / `SnapshotGroundItemExporter` now land beside the account/quest/login-ticket/item-template/static-content seams.
3. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
4. Optionally migrate selected content-bundle / NPC gameplay tests onto hermetic MemoryStores once callers want less temp-dir coupling.
