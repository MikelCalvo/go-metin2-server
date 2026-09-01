# Character point-state SQL import scoped replace GREEN — 2026-09-01

## Objective

Implement the opt-in tip-`0011` (`character_point_state` / `character_points`)
**scoped replace** path frozen in
[character point-state import replace/upsert contract freeze](2026-09-01-character-point-state-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained selected-character point-vector export
without insert-only primary-key conflicts.

## Contract shipped

1. Default `ImportCharacterPointState(...)` remains insert-only.
2. `ImportCharacterPointState(..., ImportCharacterPointStateOptions{Replace: true})`
   deletes tip-`0011` child rows for each quarantined `character_id`, then
   inserts the canonicalized export rows inside one transaction.
3. Optional export `character_ids` merges with point row-derived ids so a listed
   character with zero point rows can wipe-to-empty; empty `character_ids` plus
   empty points is a no-op mutation after schema/quarantine preflight.
4. When a character contributes any `points` rows, quarantine still requires the
   complete fixed-width `0..254` vector (255 rows). Sparse / duplicate /
   out-of-range vectors stay fail-closed.
5. Characters not listed remain untouched (no global truncate).
6. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0003` `character-item-state`,
   tip-`0004` `character-quest-state`, tip-`0011` `character-point-state`, and
   tip-`0015` `character-safebox-state`. Other kinds reject the replace
   confirmation as usage.
7. Successful replace stdout includes metadata-only
   `CharacterPointStateImportResult` with `replaced: true`.
8. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no replace for other tip kinds.

## Proof

- `go test ./internal/accountstore -run 'CharacterPointState|ImportCharacterPointState|ValidateCharacterPointState|QuarantineCharacterPointState'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/accountstore -run 'SQLiteHarnessPointStateImport'`

## Status

GREEN on `lane/persistence`. Follow-on tip-`0023` myshop unit-prices
scoped-replace GREEN is owned by
[character myshop unit-prices import scoped replace GREEN](2026-09-01-character-myshop-unit-prices-import-scoped-replace-green.md);
production-engine selection remains deferred.
