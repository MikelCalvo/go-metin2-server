# Character item-state SQL import scoped replace GREEN — 2026-08-31

## Objective

Implement the opt-in tip-`0003` (`character_item_state`, including additive
`0024` sockets + `0027` attributes) **scoped replace** path frozen in
[character item-state import replace/upsert contract freeze](2026-08-31-character-item-state-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained export without insert-only primary-key
conflicts.

## Contract shipped

1. Default `ImportCharacterItemState(...)` remains insert-only.
2. `ImportCharacterItemState(..., ImportCharacterItemStateOptions{Replace: true})`
   deletes tip-`0003` child rows for each quarantined `character_id`, then
   inserts the canonicalized export rows inside one transaction.
3. Optional export `character_ids` merges with row-derived ids so a listed
   character with zero child rows can wipe-to-empty; empty `character_ids` plus
   empty rows is a no-op mutation after schema/quarantine preflight.
4. Characters not listed remain untouched (no global truncate).
5. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` is tip-`0003` `character-item-state` only.
   Other kinds reject the replace confirmation as usage.
6. Successful replace stdout includes metadata-only
   `CharacterItemStateImportResult` with `replaced: true`.
7. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no replace for other tip kinds.

## Proof

- `go test ./internal/accountstore -run 'CharacterItemState|ImportCharacterItemState|ValidateCharacterItemState|QuarantineCharacterItemState'`
- `go test ./internal/migratecli -run ImportExport`
- `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessItemStateImport`

## Status

GREEN on `lane/persistence`. Follow-on tip-`0015` safebox scoped-replace is now
owned by [character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md)
(freeze: [character safebox-state import replace/upsert contract freeze](2026-09-01-character-safebox-state-import-replace-upsert-contract-freeze.md));
other tip-kind freezes and production-engine selection remain deferred.
