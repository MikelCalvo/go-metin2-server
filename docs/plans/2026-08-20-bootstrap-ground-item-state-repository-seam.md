# Bootstrap Ground-Item-State Repository Seam — 2026-08-20

## Objective

Land the sixth narrow repository-style seam for pending bootstrap ground item / ground gold handles already projected onto migration `0010_bootstrap_ground_item_state`, so export and future backfill/recovery tooling can target a named interface instead of anonymous `[]GroundItemSnapshot` / free-function coupling, and hermetic tests can prove the migration-shaped export boundary without requiring a live shared-world registration path or claiming restart durability.

## Contract frozen by this slice

1. `worldruntime.BootstrapGroundItemStateExporter` exposes:
   - `ExportBootstrapGroundItemState() (BootstrapGroundItemStateExport, error)`
2. New hermetic `worldruntime.MemoryGroundItemStore` satisfies that interface from an in-memory snapshot list:
   - no filesystem writes
   - no SQL / DSN
   - deep-copies snapshots on Replace/List/Export
   - empty / unset store returns an empty migration-shaped export
   - deliberately omits backup / restore / process-restart recovery
3. Thin `worldruntime.SnapshotGroundItemExporter` adapts any `func() []GroundItemSnapshot` (including live shared-world reads) onto the same named interface without opening a database.
4. `gameRuntime.ExportBootstrapGroundItemState` prefers an injected `BootstrapGroundItemStateExporter` when present; otherwise it continues to project live `GroundItems()` through the existing free-function validator.
5. Existing quarantine validators remain the fail-closed preflight for retained export artifacts. This slice does **not** make ground handles durable across process restart and does **not** add import/backfill into live worlds or databases.

## What this is not yet

- SQL-backed repository implementation
- DB connection pool / driver selection for runtime stores
- INSERT / backfill / restore-from-export tooling
- process-restart restoration of pending ground handles
- ownership timer / public-release policy persistence
- remote admin API

## TDD and validation

Focused coverage:

- `go test ./internal/worldruntime -run 'MemoryGroundItemStore|BootstrapGroundItemStateExporter|SnapshotGroundItemExporter' -count=1`
- `go test ./internal/minimal -run 'ExportsBootstrapGroundItemStateThroughMemoryStoreSeam|BootstrapGroundItemStateExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
2. Decide whether in-memory ground handles should survive process restart before adding recovery that consumes quarantined `0010` exports.
3. Keep richer world-state persistence (occupancy timers, ownership release, multi-map durability) deferred until this seam and operator quarantine path stay stable.
