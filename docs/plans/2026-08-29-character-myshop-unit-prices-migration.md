# Character MyShop Unit-Prices Migration — 2026-08-29

## Objective

Close the migration/export/import gap after items-lane owned durable FileStore
`myshop_unit_prices`: add catalog migration `0023_character_myshop_unit_prices`,
project remembered silk-bag unit prices through a new tip-`0023` export /
quarantine / import seam, and fail closed before SQL INSERT when the ledger does
not own version `23`.

## Why now

- FileStore / runtime already round-trip and rematerialize `myshop_unit_prices`
  across reconnect / restart (`docs/plans/2026-08-29-myshop-unit-prices-durable-filestore.md`).
- Migration-shaped roster / item-state / point-state exports still omit the
  field, so quarantined SQL backfill silently drops remembered silk pricelists.
- Track E prefers explicit additive schema + import preflight over opaque
  driver errors (same spirit as `0015` safebox money and `0021`/`0022` refine
  companions).
- Safer than inventory/equipment instance-socket SQL this run: sockets would
  tip-bump or additive-gate the already-imported `0003` surface.

## Contract frozen by this slice

1. Embedded catalog adds `0023_character_myshop_unit_prices` after
   `0022_item_template_refine_fail_result_vnum` (catalog tip moves to `23`).
2. `up` creates child table `character_myshop_unit_prices` with
   `PRIMARY KEY (character_id, vnum)`, FK to `characters(id)`, and CHECKs:
   - `character_id > 0`
   - `vnum > 0 AND vnum <= 4294967295`
   - `unit_price >= 0 AND unit_price <= 4294967295`
3. `down` drops the table.
4. Keep roster export tip at `0002` / `account_character_roster`. New export /
   quarantine / import identity is `23` / `character_myshop_unit_prices`.
5. `accountstore` owns:
   - `ExportCharacterMyShopUnitPrices` (omit empty characters; ≤40 rows /
     character; sorted by `(character_id, vnum)`; fail closed on zero/dup vnum)
   - `Validate` / `QuarantineCharacterMyShopUnitPricesExport`
   - `ImportCharacterMyShopUnitPrices` (quarantine → require ledger tip `23` →
     parameterized INSERT only; no upsert)
6. Loopback routes:
   - `GET /local/account-store/exports/character-myshop-unit-prices`
   - `POST /local/account-store/exports/character-myshop-unit-prices/quarantine`
7. CLI kinds `quarantine-export` / `import-export` / drills / status gain
   `character-myshop-unit-prices`.
8. Upsert / stock production driver / GD `MYSHOP_PRICELIST_*` / instance-socket
   SQL remain explicitly deferred.

## What this is not yet

- retipping roster exports to include prices inline
- DB-backed live myshop repository / runtime loading
- inventory/equipment instance socket columns on `0003`
- remote admin / daemon mutation route / secrets in git
- GD/DB `MYSHOP_PRICELIST_*` packets

## Likely files to change

- `db/migrations/0023_character_myshop_unit_prices.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/accountstore/myshop_unit_prices_*.go`
- `internal/ops/pprofmux.go` (+ tests)
- `internal/migratecli/*`
- `internal/minimal/gamed_migration_ops.go` / factory / tests
- `docs/development.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- this plan

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/accountstore -run 'MyShopUnitPrices|CharacterMyShop' -count=1`
- `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessMyShopUnitPrices -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport|ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus|MyShopUnitPrices|CharacterMyShop' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration|MyShopUnitPrices' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep upsert / stock production driver deferred.
2. Keep inventory/equipment instance-socket SQL deferred until that FileStore
   surface needs a quarantine companion.
3. Keep GD/DB `MYSHOP_PRICELIST_*` deferred on the items lane.
