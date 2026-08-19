# Static Actor Content-State Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0008_static_actor_content_state` migration-shaped export so operators can verify a retained authored static-actor / interaction-definition JSON artifact before any future DB backfill or repository work mutates stores.

## Contract frozen by this slice

1. `staticstore.ValidateStaticActorContentStateExport(...)` accepts only:
   - `migration_version == 8`
   - `migration_name == "static_actor_content_state"`
   - non-nil `interaction_definitions`, `merchant_catalog_entries`, `static_actors`, and `reward_drops` slices (empty collections are valid)
   - rows that round-trip through the same fail-closed `ExportStaticActorContentState(...)` validator already owned by the live export path
   - interaction kinds limited to the historical `0008` set (`info`, `talk`, `warp`, `shop_preview`); newer runtime kinds such as `quest_flag` remain rejected
2. Successful validation returns a metadata-only quarantine summary:
   - `interaction_definition_count`
   - `merchant_catalog_entry_count`
   - `static_actor_count`
   - `reward_drop_count`
   - deterministic `entity_ids` in canonical actor order
   - deterministic sorted `interaction_kinds`
3. `staticstore.QuarantineStaticActorContentStateExport(...)` validates, then returns the same summary plus a canonicalized export ordered exactly like the live content-state exporter.
4. Loopback-only `POST /local/static-actors/exports/static-actor-content-state/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never writes static-actor or interaction snapshots, never emits SQL

## What this is not yet

- DB INSERT / backfill execution
- content-store mutation or restore from export rows
- a repository seam
- quarantine for item-template / ground-item exports
- a new migration that widens `0008` to own `quest_flag` columns
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/staticstore -run 'StaticActorContentState' -count=1`
- `go test ./internal/ops -run 'StaticActorContentStateQuarantine' -count=1`

## Follow-up options

1. Extend the same quarantine pattern to item-template and bootstrap ground-item exports.
2. Add CLI-only quarantine inspection beside `metin2-migrate`.
3. Extract a repository seam only after quarantine preflight proves the boundary.
4. Widen the migration/export contract deliberately when `quest_flag` and kill-quest credit columns become DB-owned.
