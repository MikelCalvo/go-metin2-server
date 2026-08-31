# Safebox cell instance-attributes SQL additive — 2026-08-30

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
- Migration-shaped tip-`0015` export/import already owns additive `0025`
  sockets but still omitted attributes, so quarantined SQL backfill silently
  dropped authoritative safebox cell instance attributes.
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as tip-`0003`+`0024`+`0027` and
  tip-`0015`+`0025`).
- Safer than inventing a new tip identity: attributes extend existing
  `character_safebox_items` rows while export identity stays tip `15`.

## Contract frozen by this slice

1. Embedded catalog adds `0028_character_safebox_item_instance_attributes` after
   `0027_character_item_instance_attributes` (catalog tip moves to `28`).
2. `up` adds `has_attributes` + `attr0_type`/`attr0_value` … `attr6_type` /
   `attr6_value` on `character_safebox_items` with CHECKs mirroring `0027`:
   - `has_attributes IN (0, 1)`
   - each type in `0..255`, each value in signed int16 range
   - when `has_attributes = 0`, all attr types/values must be `0`
3. `down` drops those columns (dependent `attr6_value` first).
4. Keep tip-`0015` / `character_safebox_money` as the export / quarantine /
   import-result migration identity (do **not** retip to `28`).
5. `CharacterSafeboxItemRow` carries optional `has_attributes` + attr fields;
   export maps:
   - `HasAttributes == false` / omitted → omitted / `has_attributes=false`,
     attrs `0`
   - `HasAttributes == true` (including all-zero / type-zero) →
     `has_attributes=true` + values
6. Quarantine rejects non-zero attribute type/value when `has_attributes` is
   false.
7. `ImportCharacterSafeboxState` inserts the new columns and requires tip-`0015`
   plus additive `0025` plus additive `0028` before any INSERT
   (`ErrCharacterSafeboxStateImportSchemaRequired` when any boundary is
   missing).
8. Mall / TMP4 CG `SAFEBOX_MONEY` request header / client
   `SAFEBOX_CHANGE_PASSWORD` packets / tip-`0010` attribute SQL / seeded
   attribute tip-sync remain deferred.

## What this is not yet

- retipping safebox-state exports to `migration_version=28`
- DB-backed live safebox repositories
- tip-`0010` ground attribute SQL companion
- seeded hermetic import-export drill attribute rows for tip-`0015`
- remote admin / daemon mutation route / secrets in git

## Likely files changed

- `db/migrations/0028_character_safebox_item_instance_attributes.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/safeboxstore/export.go`
- `internal/safeboxstore/export_quarantine.go`
- `internal/safeboxstore/safebox_state_import.go`
- `internal/safeboxstore/*_test.go` (+ sqlite harness)
- `internal/migratecli` / `internal/ops` / `internal/minimal` migration tips
- `docs/development.md` / migration contract / roadmap / debugging tip-sync / QA
- this plan

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/safeboxstore -run 'ExportCharacterSafebox|ValidateCharacterSafebox|QuarantineCharacterSafebox|ImportCharacterSafebox|InstanceAttributes|InstanceSockets' -count=1`
- `go test -tags=sqlite_harness ./internal/safeboxstore -run SQLiteHarnessSafeboxStateImport -count=1`
- `go test ./internal/migratecli -run 'ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Status

GREEN — additive catalog tip `0028` projects presence-aware tip-`0015`
instance attributes through export/quarantine/import with ledger preflight
requiring tip-`0015` + `0025` + `0028`.

## Follow-up options

1. ~~Seed presence-aware safebox cell attributes in the hermetic import-export
   drill + tip-sync loopback character-safebox-state docs.~~ Done:
   [seeded safebox cell instance-attributes tip
   sync](2026-08-31-seeded-safebox-cell-instance-attributes-import-export-drill.md).
2. Prefer tip-`0010` ground attribute SQL companion next, or keep that deferred
   until operators choose the additive companion after durable FileStore
   rematerialize (already owned).
3. Keep upsert / stock production driver deferred.
4. Keep tip-`0015` export identity until a deliberate retip is needed.
