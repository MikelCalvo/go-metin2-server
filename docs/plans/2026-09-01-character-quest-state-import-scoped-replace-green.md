# Character quest-state SQL import scoped replace GREEN — 2026-09-01

## Objective

Implement the opt-in tip-`0004` (`character_quest_state` / `character_quest_flags`)
**scoped replace** path frozen in
[character quest-state import replace/upsert contract freeze](2026-09-01-character-quest-state-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained quest-flag export without insert-only
primary-key conflicts.

## Contract shipped

1. Default `ImportCharacterQuestState(...)` remains insert-only.
2. `ImportCharacterQuestState(..., ImportCharacterQuestStateOptions{Replace: true})`
   deletes tip-`0004` child rows for each quarantined `character_id`, then
   inserts the canonicalized export rows inside one transaction.
3. Optional export `character_ids` merges with flag row-derived ids so a listed
   character with zero flag rows can wipe-to-empty; empty `character_ids` plus
   empty flags is a no-op mutation after schema/quarantine preflight.
4. Characters not listed remain untouched (no global truncate).
5. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0003` `character-item-state`,
   tip-`0004` `character-quest-state`, and tip-`0015` `character-safebox-state`.
   Other kinds reject the replace confirmation as usage.
6. Successful replace stdout includes metadata-only
   `CharacterQuestStateImportResult` with `replaced: true`.
7. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no replace for other tip kinds.

## Proof

- `go test ./internal/queststate -run 'CharacterQuestState|ImportCharacterQuestState|ValidateCharacterQuestState|QuarantineCharacterQuestState'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/queststate -run 'SQLiteHarnessQuestStateImport'`

## Status

GREEN on `lane/content`. Other tip-kind freezes and production-engine selection
remain deferred.
