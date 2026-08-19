# Character Quest-State Export Quarantine — 2026-08-19

## Objective

Add a fail-closed import/quarantine preflight for the existing `0004_character_quest_state` migration-shaped export so operators can verify a retained quest-flag JSON artifact before any future DB backfill or repository work mutates stores.

## Contract frozen by this slice

1. `queststate.ValidateCharacterQuestStateExport(...)` accepts only:
   - `migration_version == 4`
   - `migration_name == "character_quest_state"`
   - non-nil `flags` slice (empty is valid for zero characters)
   - `character_id > 0`
   - bootstrap-valid `character`, `quest_ref`, and `flag` identities
   - `value > 0`
   - unique `(character_id, quest_ref, flag)` keys
   - stable character-id ↔ character-name mapping within one export
2. Successful validation returns a metadata-only quarantine summary:
   - `character_count`
   - `flag_count`
   - deterministic sorted `character_ids`
3. `queststate.QuarantineCharacterQuestStateExport(...)` validates, then returns the same summary plus a canonicalized export ordered by ascending `character_id`, then `quest_ref`, then `flag`.
4. Loopback-only `POST /local/quest-state/exports/character-quest-state/quarantine` on `gamed`:
   - accepts the export JSON body
   - rejects non-loopback callers with `403`
   - rejects wrong methods with `405`
   - rejects oversized / malformed / invalid UTF-8 bodies with `400` / `413`
   - returns `409` when the payload fails quarantine validation
   - never opens a database, never writes quest-state snapshots, never emits SQL

## What this is not yet

- DB INSERT / backfill execution
- quest-state mutation or restore from export rows
- a repository seam
- quarantine for roster / login-ticket / item-template / static-content exports
- remote admin auth

## TDD and validation

Focused coverage:

- `go test ./internal/queststate -run 'CharacterQuestState' -count=1`
- `go test ./internal/ops -run 'CharacterQuestStateQuarantine' -count=1`

## Follow-up options

1. Extend the same quarantine pattern to roster / login-ticket / item-template / static-content exports.
2. Add CLI-only quarantine inspection beside `metin2-migrate`.
3. Extract a repository seam only after quarantine preflight proves the boundary.
