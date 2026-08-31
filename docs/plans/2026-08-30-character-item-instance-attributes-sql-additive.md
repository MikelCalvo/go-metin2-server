# Character Item Instance-Attributes SQL Additive — 2026-08-30

## Objective

Close the migration/export/import gap after items-lane owned FileStore
per-instance inventory/equipment attributes: add additive catalog migration
`0027_character_item_instance_attributes`, project presence-aware attributes
through tip-`0003` export/quarantine/import, and fail closed before SQL INSERT
when the ledger owns tip-`0003` + additive `0024` but not additive `0027`.

## Why now

- FileStore / runtime already round-trip and honor instance attributes
  (including explicit all-zero / type-zero) for encode preference surfaces —
  see [attributes-on-instance FileStore + encode GREEN](2026-08-30-attributes-on-instance-filestore-encode-green.md).
- Migration-shaped tip-`0003` export/import already owns additive `0024`
  sockets but still omitted attributes, so quarantined SQL backfill silently
  dropped authoritative instance attributes.
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as tip-`0003` + `0024`).
- Safer than inventing a new tip identity: attributes extend existing
  `character_inventory_items` / `character_equipment_items` rows.

## Contract frozen by this slice

1. Embedded catalog adds `0027_character_item_instance_attributes` after
   `0026_bootstrap_ground_item_instance_sockets` (catalog tip moves to `27`).
2. `up` adds `has_attributes` + `attr0_type`/`attr0_value` … `attr6_type` /
   `attr6_value` on both inventory and equipment tables with CHECKs:
   - `has_attributes IN (0, 1)`
   - each type in `0..255`, each value in signed int16 range
   - when `has_attributes = 0`, all attr types/values must be `0`
3. `down` drops those columns (dependent `attr6_value` first).
4. Keep tip-`0003` / `character_item_state` as the export / quarantine /
   import-result migration identity (do **not** retip to `27`).
5. `CharacterInventoryItemRow` / `CharacterEquipmentItemRow` carry optional
   `has_attributes` + attr fields; export maps:
   - `Attributes == nil` → omitted / `has_attributes=false`, attrs `0`
   - `Attributes != nil` (including all-zero / type-zero) → `has_attributes=true` + values
6. Quarantine rejects non-zero attribute type/value when `has_attributes` is false.
7. `ImportCharacterItemState` inserts the new columns and requires tip-`0003`
   plus additive `0024` plus additive `0027` before any INSERT
   (`ErrCharacterItemStateImportSchemaRequired`).
8. tip-`0010`/`0015` attribute companions, upsert, seeded drill tip-sync,
   attribute gameplay, and remote admin remain explicitly deferred.

## What this is not yet

- retipping item-state exports to `migration_version=27`
- tip-`0010` / tip-`0015` attribute SQL companions
- DB-backed live inventory/equipment repositories
- seeded hermetic import-export drill attribute rows
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `db/migrations/0027_character_item_instance_attributes.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/accountstore/item_state_export.go`
- `internal/accountstore/item_state_quarantine.go`
- `internal/accountstore/item_state_import.go`
- `internal/accountstore/*_test.go` (+ sqlite harness)
- `internal/ops/pprofmux_test.go`
- `internal/minimal/factory_test.go` / `gamed_migration_ops_test.go`
- `internal/migratecli/import_export_test.go`
- `docs/development.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- this plan

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummary|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/accountstore -run 'ExportCharacterItemState|ValidateCharacterItemState|QuarantineCharacterItemState|ImportCharacterItemState|ItemState' -count=1`
- `go test -tags=sqlite_harness ./internal/accountstore -run SQLiteHarnessItemStateImport -count=1`
- `go test ./internal/migratecli -run 'ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Status

GREEN — additive catalog tip `0027` projects presence-aware tip-`0003`
instance attributes through export/quarantine/import with ledger preflight
requiring tip-`0003` + `0024` + `0027`.

## Follow-up options

1. ~~Seed presence-aware instance attributes in the hermetic import-export drill
   + tip-sync loopback character-item-state docs.~~ Done — see [seeded item
   instance-attributes tip sync](2026-08-31-seeded-item-instance-attributes-import-export-drill.md).
2. ~~Prefer tip-`0015` attribute SQL companion after durable safebox attribute
   rematerialize.~~ Docs freeze owned — see [safebox cell instance-attributes SQL
   additive](2026-08-31-safebox-cell-instance-attributes-sql-additive.md). tip-`0010`
   ground attribute companion is now owned — see
   [bootstrap ground-item instance-attributes SQL additive](2026-08-31-bootstrap-ground-item-instance-attributes-sql-additive.md)
   and [seeded ground-item instance-attributes tip sync](2026-08-31-seeded-ground-item-instance-attributes-import-export-drill.md).
3. Keep upsert / stock production driver deferred.
4. Keep tip-`0003` export identity until a deliberate retip is needed.
