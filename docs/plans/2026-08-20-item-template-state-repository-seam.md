# Item-Template-State Repository Seam — 2026-08-20

## Objective

Land the fourth narrow repository-style seam for durable authored item-template state already projected onto migration `0009_item_template_refine_info` (after `0005` base schema and `0006` safebox-reject storage), so export and future backfill tooling can target a named interface instead of anonymous `FileStore` type asserts, and hermetic tests can prove reduced bootstrap filesystem coupling without claiming a DB-backed runtime.

## Contract frozen by this slice

1. `itemstore.ItemTemplateStateExporter` exposes:
   - `ExportItemTemplateState() (ItemTemplateStateExport, error)`
2. `FileStore` continues to satisfy `Store` and `ItemTemplateStateExporter` with no disk-path behavior change.
3. New hermetic `MemoryStore` also satisfies those two interfaces from an in-memory snapshot:
   - no filesystem writes
   - no SQL / DSN
   - deep-copies templates (including use/equip effects and refine materials) on Load/Save/Export
   - missing committed snapshot returns `ErrSnapshotNotFound` from `Load` and an empty migration-shaped export from `ExportItemTemplateState`
   - deliberately omits backup / restore / crash-temp cleanup
4. `MemoryStore` owns hermetic `Load` / `Save` so content-bundle and merchant/item policy tests can exercise authored templates without a disk-backed store.
5. `gameRuntime.ExportItemTemplateState` asserts the named `ItemTemplateStateExporter` interface instead of an anonymous method set.
6. Existing quarantine validators remain the fail-closed preflight for retained export artifacts. This slice does **not** add import/backfill into live stores or databases.

## What this is not yet

- SQL-backed repository implementation
- DB connection pool / driver selection for runtime stores
- INSERT / backfill / restore-from-export tooling
- broadening `Store` into backup/restore/crash-temp
- static-content / ground-item repository seams
- ground-item restart durability
- remote admin API

## TDD and validation

Focused coverage:

- `go test ./internal/itemstore -run 'MemoryStore|ItemTemplateStateExporter' -count=1`
- `go test ./internal/minimal -run 'ExportsItemTemplateStateThroughMemoryStoreSeam|ItemTemplateStateExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Add matching hermetic seams for static-content exports once callers need the same coupling reduction. Account/quest/login-ticket/item-template seams are now landed.
2. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
3. Keep ground-item restart durability deferred until a world-state repository exists.
