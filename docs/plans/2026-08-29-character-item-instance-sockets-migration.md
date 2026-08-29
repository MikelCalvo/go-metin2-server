# Character Item Instance-Sockets Migration — 2026-08-29

## Objective

Close the migration/export/import gap after items-lane owned FileStore
per-instance inventory/equipment sockets: add additive catalog migration
`0024_character_item_instance_sockets`, project presence-aware sockets through
tip-`0003` export/quarantine/import, and fail closed before SQL INSERT when the
ledger owns tip-`0003` but not additive `0024`.

## Why now

- FileStore / runtime already round-trip and honor instance sockets (including
  explicit zero) for MYSHOP auto-potion deactivate, exchange display, and
  carried refresh encode (`docs/plans/2026-08-28-myshop-open-auto-potion-socket0-deactivate.md`).
- Migration-shaped tip-`0003` export/import still omitted sockets after that
  GREEN, so quarantined SQL backfill silently dropped authoritative instance
  sockets (including deactivated auto-potion `socket0 = 0`).
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as tip-`0009` + `0021`/`0022`).
- Safer than inventing a new tip identity: sockets extend existing
  `character_inventory_items` / `character_equipment_items` rows.

## Contract frozen by this slice

1. Embedded catalog adds `0024_character_item_instance_sockets` after
   `0023_character_myshop_unit_prices` (catalog tip moves to `24`).
2. `up` adds `has_sockets` + `socket0`/`socket1`/`socket2` on both
   `character_inventory_items` and `character_equipment_items` with CHECKs:
   - `has_sockets IN (0, 1)`
   - each socket in signed int32 range
   - when `has_sockets = 0`, all sockets must be `0`
3. `down` drops those columns (dependent `socket2` first).
4. Keep tip-`0003` / `character_item_state` as the export / quarantine /
   import-result migration identity (do **not** retip to `24`).
5. `CharacterInventoryItemRow` / `CharacterEquipmentItemRow` carry optional
   `has_sockets` + `socket0`/`socket1`/`socket2`; export maps:
   - `Sockets == nil` → omitted / `has_sockets=false`, sockets `0`
   - `Sockets != nil` (including all-zero) → `has_sockets=true` + values
6. Quarantine rejects non-zero sockets when `has_sockets` is false.
7. `ImportCharacterItemState` inserts the new columns and requires tip-`0003`
   plus additive `0024` before any INSERT
   (`ErrCharacterItemStateImportSchemaRequired`).
8. Upsert / stock production driver / GD/DB myshop pricelist / quest-running
   remain explicitly deferred.

## What this is not yet

- retipping item-state exports to `migration_version=24`
- DB-backed live inventory/equipment repositories
- remote admin / daemon mutation route / secrets in git
- quest-running MYSHOP open block / shopkeeper polymorph / GD `MYSHOP_PRICELIST_*`

## Likely files to change

- `db/migrations/0024_character_item_instance_sockets.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/accountstore/item_state_export.go`
- `internal/accountstore/item_state_quarantine.go`
- `internal/accountstore/item_state_import.go`
- `internal/accountstore/*_test.go` (+ sqlite harness)
- `internal/ops/pprofmux_test.go`
- `internal/minimal/factory_test.go` / `gamed_migration_ops_test.go`
- `docs/development.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- this plan

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/accountstore -run 'ExportCharacterItemState|ValidateCharacterItemState|QuarantineCharacterItemState|ImportCharacterItemState|ItemState' -count=1`
- `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessItemStateImport -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport|ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus|CharacterItemState' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep upsert / stock production driver deferred.
2. Keep tip-`0003` export identity until a deliberate retip is needed.
3. Prefer quest-running MYSHOP open block / GD pricelist only with a fresh
   client-visible evidence freeze.
