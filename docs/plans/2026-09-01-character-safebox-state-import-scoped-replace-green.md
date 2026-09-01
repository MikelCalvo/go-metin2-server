# Character safebox-state SQL import scoped replace GREEN — 2026-09-01

## Objective

Implement the opt-in tip-`0015` (`character_safebox_money`, including additive
`0025` sockets + `0028` attributes) **scoped replace** path frozen in
[character safebox-state import replace/upsert contract freeze](2026-09-01-character-safebox-state-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained warehouse export without insert-only
primary-key conflicts.

## Contract shipped

1. Default `ImportCharacterSafeboxState(...)` remains insert-only.
2. `ImportCharacterSafeboxState(..., ImportCharacterSafeboxStateOptions{Replace: true})`
   deletes tip-`0015` child rows for each quarantined `character_id`, then
   inserts the canonicalized export rows inside one transaction.
3. Optional export `character_ids` merges with password/item row-derived ids so a
   listed character with zero password and zero item rows can wipe-to-empty;
   empty `character_ids` plus empty rows is a no-op mutation after
   schema/quarantine preflight.
4. Characters not listed remain untouched (no global truncate).
5. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0003` `character-item-state` and
   tip-`0015` `character-safebox-state`. Other kinds reject the replace
   confirmation as usage.
6. Successful replace stdout includes metadata-only
   `CharacterSafeboxStateImportResult` with `replaced: true`.
7. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no replace for other tip kinds.

## Proof

- `go test ./internal/safeboxstore -run 'CharacterSafeboxState|ImportCharacterSafeboxState|ValidateCharacterSafeboxState|QuarantineCharacterSafeboxState'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/safeboxstore -run 'SQLiteHarnessSafeboxStateImport'`

## Status

GREEN on `lane/items` (now on `main`). Follow-on tip-`0011` point-state
scoped-replace freeze is owned by
[character point-state import replace/upsert contract freeze](2026-09-01-character-point-state-import-replace-upsert-contract-freeze.md);
production-engine selection remains deferred.
