# Character myshop unit-prices SQL import scoped replace GREEN — 2026-09-01

## Objective

Implement the opt-in tip-`0023` (`character_myshop_unit_prices`)
**scoped replace** path frozen in
[character myshop unit-prices import replace/upsert contract freeze](2026-09-01-character-myshop-unit-prices-import-replace-upsert-contract-freeze.md)
so operators can re-backfill a retained silk-bag remembered unit-price export
without insert-only primary-key conflicts.

## Contract shipped

1. Default `ImportCharacterMyShopUnitPrices(...)` remains insert-only.
2. `ImportCharacterMyShopUnitPrices(..., ImportCharacterMyShopUnitPricesOptions{Replace: true})`
   deletes tip-`0023` child rows for each quarantined `character_id`, then
   inserts the canonicalized export rows inside one transaction.
3. Optional export `character_ids` merges with unit-price row-derived ids so a
   listed character with zero price rows can wipe-to-empty; empty
   `character_ids` plus empty `unit_prices` is a no-op mutation after
   schema/quarantine preflight.
4. Quarantine continues to reject zero `character_id`, zero `vnum`, duplicate
   `(character_id, vnum)`, and per-character row counts above
   `loginticket.MyShopUnitPriceMax`.
5. Characters not listed remain untouched (no global truncate).
6. CLI: `metin2-migrate import-export ... --i-confirm-sql-import
   --i-confirm-scoped-replace` accepts tip-`0003` `character-item-state`,
   tip-`0004` `character-quest-state`, tip-`0011` `character-point-state`,
   tip-`0015` `character-safebox-state`, and tip-`0023`
   `character-myshop-unit-prices`. Other kinds reject the replace confirmation
   as usage.
7. Successful replace stdout includes metadata-only
   `CharacterMyShopUnitPricesImportResult` with `replaced: true`.
8. Still no stock production driver, no daemon mutation route, no catalog tip
   `0030`, and no replace for other tip kinds.

## Proof

- `go test ./internal/accountstore -run 'MyShopUnitPrices|ImportCharacterMyShopUnitPrices|ValidateCharacterMyShopUnitPrices|QuarantineCharacterMyShopUnitPrices'`
- `go test ./internal/migratecli -run 'ImportExport.*ScopedReplace|ImportExportRejectsUsage'`
- `go test -tags=sqlite_harness ./internal/accountstore -run 'SQLiteHarnessMyShopUnitPricesImport'`

## Status

GREEN on `lane/persistence`. Follow-on tip-`0002` roster scoped-replace GREEN is owned by
[account/character roster import scoped replace GREEN](2026-09-02-account-character-roster-import-scoped-replace-green.md);
production-engine selection remains deferred.
