# Safebox cell instance-attributes SQL additive — 2026-08-31

## Objective

Close the migration/export/import gap after items-lane owned FileStore
presence-aware safebox cell instance attributes: add additive catalog migration
`0028_character_safebox_item_instance_attributes`, project those attributes
through tip-`0015` export/quarantine/import, and fail closed before SQL INSERT
when the ledger owns tip-`0015` + additive `0025` but not additive `0028`.

## Why now

- Durable safebox FileStore / runtime already round-trip and honor cell instance
  attributes (including explicit all-zero / type-zero) through check-in →
  restart → reopen / checkout
  (`docs/plans/2026-08-30-safebox-cell-instance-attributes-durable.md`).
- Tip-`0003` inventory/equipment already owns additive `0027` attributes +
  seeded hermetic tip sync
  (`docs/plans/2026-08-30-character-item-instance-attributes-sql-additive.md`,
  `docs/plans/2026-08-31-seeded-item-instance-attributes-import-export-drill.md`).
- Migration-shaped tip-`0015` export/import still omit attributes after that
  GREEN, so quarantined SQL backfill silently drops authoritative safebox cell
  instance attributes.
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as tip-`0015` + `0025` sockets
  and tip-`0003` + `0027` attributes).
- Safer than inventing a new tip identity: attributes extend existing
  `character_safebox_items` rows while export identity stays tip `15`.

## Contract frozen by this slice

1. Embedded catalog adds `0028_character_safebox_item_instance_attributes` after
   `0027_character_item_instance_attributes` (catalog tip moves to `28`).
2. `up` adds `has_attributes` + `attr0_type`/`attr0_value` … `attr6_type` /
   `attr6_value` on `character_safebox_items` with CHECKs mirroring `0027`:
   - `has_attributes IN (0, 1)`
   - each attr type in `[0, 255]` and value in signed int16 range
   - when `has_attributes = 0`, all attr types/values must be `0`
3. `down` drops those columns (dependent `attr6_*` first).
4. Keep tip-`0015` / `character_safebox_money` as the export / quarantine /
   import-result migration identity (do **not** retip to `28`).
5. `CharacterSafeboxItemRow` carries optional `has_attributes` +
   `attr0_type`/`attr0_value` … `attr6_type`/`attr6_value`; export maps:
   - FileStore `HasAttributes == false` / omitted / nil attributes →
     omitted / `has_attributes=false`, attrs `0`
   - FileStore `HasAttributes == true` (including all-zero / type-zero) →
     `has_attributes=true` + values
6. Quarantine rejects non-zero attr types/values when `has_attributes` is false.
7. `ImportCharacterSafeboxState` inserts the new columns and requires tip-`0015`
   plus additive `0025` plus additive `0028` before any INSERT
   (`ErrCharacterSafeboxStateImportSchemaRequired` when any required boundary is
   missing).
8. Mall / TMP4 CG `SAFEBOX_MONEY` request header / client
   `SAFEBOX_CHANGE_PASSWORD` packets / tip-`0010` ground attribute companions /
   refine catalysts remain deferred.

## What this is not yet

- retipping safebox-state exports to `migration_version=28`
- DB-backed live safebox repositories
- remote admin / daemon mutation route / secrets in git
- mall open/checkout / GD/DB myshop
- tip-`0010` ground-item attribute SQL companion (separate follow-on)

## Likely files to change (GREEN follow-on)

- `db/migrations/0028_character_safebox_item_instance_attributes.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/safeboxstore/export.go`
- `internal/safeboxstore/export_quarantine.go`
- `internal/safeboxstore/safebox_state_import.go`
- `internal/safeboxstore/*_test.go` (+ sqlite harness)
- `internal/migratecli` / `internal/ops` / `internal/minimal` migration tips
- `docs/development.md` / migration contract / roadmap / QA checklist
- this plan

## TDD and validation (after GREEN opens)

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/safeboxstore -run 'ExportCharacterSafebox|ValidateCharacterSafebox|QuarantineCharacterSafebox|ImportCharacterSafebox|InstanceAttributes' -count=1`
- `go test -tags=sqlite_harness ./internal/safeboxstore -run SQLiteHarness -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport|ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus|CharacterSafebox' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`
- `go test ./...` and `go vet ./...`

## Status

GREEN on `lane/items`: additive catalog tip `0028` projects presence-aware
tip-`0015` instance attributes through export/quarantine/import with ledger
preflight requiring tip-`0015` + `0025` + `0028`
(`feat(db): add tip-0015 safebox cell instance attributes SQL companion`).

Follow-on tip sync: seeded hermetic tip-`0015`+`0028` safebox cell attributes in
the shared `import-export-drill` is owned by
[seeded safebox cell instance-attributes tip sync](2026-08-31-seeded-safebox-cell-instance-attributes-import-export-drill.md).
