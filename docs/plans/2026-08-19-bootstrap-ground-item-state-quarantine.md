# Bootstrap Ground-Item-State Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0010_bootstrap_ground_item_state` migration-shaped export so operators can verify a retained live-ground-handle JSON artifact before any future DB backfill, crash-recovery, or repository work mutates stores.

## Contract frozen by this slice

1. `worldruntime.ValidateBootstrapGroundItemStateExport(...)` accepts only:
   - `migration_version == 10`
   - `migration_name == "bootstrap_ground_item_state"`
   - a non-nil `ground_items` slice (empty is valid)
   - rows that round-trip through the same fail-closed `ExportBootstrapGroundItemState(...)` validator already owned by the live export path
2. Successful validation returns a metadata-only quarantine summary:
   - `ground_item_count`
   - `item_shaped_count`
   - `gold_shaped_count`
   - deterministic sorted `vids`
3. `worldruntime.QuarantineBootstrapGroundItemStateExport(...)` validates, then returns the same summary plus a canonicalized export ordered by ascending visible ground `vid`.
4. Loopback-only `POST /local/ground-items/exports/bootstrap-ground-item-state/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never mutates live ground handles, never emits SQL

## What this is not yet

- DB INSERT / backfill execution
- process-restart restoration of pending ground handles
- ownership timer / public-release policy persistence
- a repository seam
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/worldruntime -run 'BootstrapGroundItemState' -count=1`
- `go test ./internal/ops -run 'BootstrapGroundItemStateQuarantine' -count=1`

## Follow-up options

1. Add CLI-only quarantine inspection beside `metin2-migrate`.
2. Decide whether in-memory ground handles should survive process restart before adding recovery.
3. Extract a repository seam only after quarantine preflight proves the boundary.
