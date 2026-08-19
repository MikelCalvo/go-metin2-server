# Character Point-State Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0011_character_point_state` migration-shaped export so operators can verify a retained point-state JSON artifact before any future DB backfill or repository work mutates stores.

## Contract frozen by this slice

1. `accountstore.ValidateCharacterPointStateExport(...)` accepts only:
   - `migration_version == 11`
   - `migration_name == "character_point_state"`
   - non-nil `points` slice (empty is valid for zero characters)
   - `character_id > 0`
   - complete contiguous `point_index` vectors `0..254` per character
   - no duplicate `(character_id, point_index)` pairs
2. Successful validation returns a metadata-only quarantine summary:
   - `character_count`
   - `point_row_count`
   - deterministic sorted `character_ids`
3. `accountstore.QuarantineCharacterPointStateExport(...)` validates, then returns the same summary plus a canonicalized export whose rows are grouped by ascending `character_id` with each character's indices `0..254`.
4. Loopback-only `POST /local/account-store/exports/character-point-state/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never writes account snapshots, never emits SQL

## What this is not yet

- DB INSERT / backfill execution
- account-store mutation or restore from point rows
- a repository seam
- quarantine for other migration-shaped exports
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/accountstore -run 'CharacterPointState' -count=1`
- `go test ./internal/ops -run 'CharacterPointStateQuarantine' -count=1`

## Follow-up options

1. Extend the same quarantine pattern to roster / item-state / quest-state exports.
2. Add CLI-only quarantine inspection beside `metin2-migrate`.
3. Extract a repository seam only after quarantine preflight proves the boundary.