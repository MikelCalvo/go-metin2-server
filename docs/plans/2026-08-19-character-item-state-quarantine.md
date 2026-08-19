# Character Item-State Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0003_character_item_state` migration-shaped export so operators can verify a retained inventory/equipment/quickslot JSON artifact before any future DB backfill or repository work mutates stores.

## Contract frozen by this slice

1. `accountstore.ValidateCharacterItemStateExport(...)` accepts only:
   - `migration_version == 3`
   - `migration_name == "character_item_state"`
   - non-nil `inventory_items`, `equipment_items`, and `quickslots` slices (empty is valid)
   - `character_id > 0`
   - inventory rows with `id > 0`, `vnum > 0`, `count > 0`, carried `slot` in `0..89`, and unique `(character_id, slot)`
   - equipment rows with `id > 0`, `vnum > 0`, `count > 0`, owned named `equip_slot`, and unique `(character_id, equip_slot)`
   - globally unique item ids across inventory and equipment rows
   - quickslot rows with unique `(character_id, position)` and migration-valid `(type, slot)` tuples
2. Successful validation returns a metadata-only quarantine summary:
   - `character_count`
   - `inventory_item_count`
   - `equipment_item_count`
   - `quickslot_count`
   - deterministic sorted `character_ids`
3. `accountstore.QuarantineCharacterItemStateExport(...)` validates, then returns the same summary plus a canonicalized export ordered by:
   - inventory: ascending `character_id`, then `slot`, then `id`
   - equipment: ascending `character_id`, then owned equipment-slot order, then `id`
   - quickslots: ascending `character_id`, then `position`, then `type`, then `slot`
4. Loopback-only `POST /local/account-store/exports/character-item-state/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never writes account snapshots, never emits SQL

## What this is not yet

- DB INSERT / backfill execution
- account-store mutation or restore from item-state rows
- a repository seam
- quarantine for roster / quest-state / login-ticket / item-template exports
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/accountstore -run 'CharacterItemState' -count=1`
- `go test ./internal/ops -run 'CharacterItemStateQuarantine' -count=1`

## Follow-up options

1. Extend the same quarantine pattern to roster / quest-state / login-ticket / item-template exports.
2. Add CLI-only quarantine inspection beside `metin2-migrate`.
3. Extract a repository seam only after quarantine preflight proves the boundary.
