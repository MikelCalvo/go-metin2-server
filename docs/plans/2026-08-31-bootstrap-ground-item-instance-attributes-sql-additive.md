# Bootstrap ground-item instance-attributes SQL additive — 2026-08-31

## Objective

Close the migration/export/import gap after items-lane owned FileStore
presence-aware pending ground instance attributes: add additive catalog migration
`0029_bootstrap_ground_item_instance_attributes`, project those attributes
through tip-`0010` export/quarantine/import, and fail closed before SQL INSERT
when the ledger owns tip-`0010` + additive `0026` but not additive `0029`.

## Why now

- Durable pending ground FileStore / runtime already round-trip and honor
  instance attributes (including explicit all-zero / type-zero) through drop →
  `gamed` restart → pickup.
- Tip-`0010` export/import already owns additive `0026` sockets
  (`docs/plans/2026-08-30-bootstrap-ground-item-instance-sockets-sql-additive.md`).
- Migration-shaped tip-`0010` export/import still omit attributes after that
  GREEN, so quarantined SQL backfill silently drops authoritative ground
  instance attributes.
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as tip-`0010`+`0026` sockets and
  tip-`0003`+`0027` / tip-`0015`+`0028` attributes).
- Safer than inventing a new tip identity: attributes extend existing
  `bootstrap_ground_items` rows while export identity stays tip `10`.

## Contract frozen by this slice

1. Embedded catalog adds `0029_bootstrap_ground_item_instance_attributes` after
   `0028_character_safebox_item_instance_attributes` (catalog tip moves to `29`).
2. `up` adds `has_attributes` + `attr0_type`/`attr0_value` … `attr6_type` /
   `attr6_value` on `bootstrap_ground_items` with CHECKs mirroring `0027`/`0028`:
   - `has_attributes IN (0, 1)`
   - each attr type in `[0, 255]` and value in signed int16 range
   - when `has_attributes = 0`, all attr types/values must be `0`
3. `down` drops those columns (dependent `attr6_*` first).
4. Keep tip-`0010` / `bootstrap_ground_item_state` as the export / quarantine /
   import-result migration identity (do **not** retip to `29`).
5. `BootstrapGroundItemStateRow` / `GroundItemSnapshot` carry optional
   `has_attributes` + `attr0_type`/`attr0_value` … `attr6_type`/`attr6_value`;
   export maps:
   - `HasAttributes == false` / omitted → omitted / `has_attributes=false`, attrs `0`
   - `HasAttributes == true` (including all-zero / type-zero) →
     `has_attributes=true` + values
6. Quarantine rejects non-zero attr types/values when `has_attributes` is false.
7. Gold-shaped rows stay attribute-less (reject `has_attributes` / non-zero attrs).
8. `ImportBootstrapGroundItemState` inserts the new columns and requires tip-`0010`
   plus additive `0026` plus additive `0029` before any INSERT
   (`ErrBootstrapGroundItemStateImportSchemaRequired` when any required boundary is
   missing).
9. Durable FileStore → tip-`0010` projection
   (`DurableGroundItemRecordsToSnapshots`) carries the same presence-aware
   attributes so operator export does not silently drop FileStore authority.
10. Upsert / stock production driver / DB-backed live ground rematerialize /
    remote admin / `ITEM_GROUND_ADD` wire attributes remain deferred.

## What this is not yet

- retipping ground-item-state exports to `migration_version=29`
- DB-backed live ground rematerialize (FileStore remains the restart path)
- remote admin / daemon mutation route / secrets in git
- changing `GC::ITEM_GROUND_ADD` / ownership wire layouts
- seeded hermetic tip-`0010`+`0029` pending ground attributes in the shared
  import-export drill (deferred follow-on)

## Likely files to change (GREEN)

- `db/migrations/0029_bootstrap_ground_item_instance_attributes.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/worldruntime/scopes.go` (`GroundItemSnapshot`)
- `internal/worldruntime/migration_export.go`
- `internal/worldruntime/migration_export_quarantine.go`
- `internal/worldruntime/ground_item_state_import.go`
- `internal/worldruntime/ground_item_file_store.go` (snapshot projection)
- `internal/worldruntime/*_test.go` (+ sqlite harness)
- `internal/migratecli` / `internal/ops` / `internal/minimal` migration tips
- `docs/development.md` / migration contract / roadmap / debugging tip-sync
- this plan

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/worldruntime -run 'ExportBootstrapGround|ValidateBootstrapGround|QuarantineBootstrapGround|ImportBootstrapGround|InstanceAttributes' -count=1`
- `go test -tags=sqlite_harness ./internal/worldruntime -run SQLiteHarnessGroundItemStateImport -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport|ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Status

GREEN — additive catalog tip `0029` projects presence-aware tip-`0010`
instance attributes through export/quarantine/import, with SQL import
requiring tip-`0010` + `0026` + `0029`.

Follow-on tip sync: seeded hermetic tip-`0010`+`0029` pending ground attributes
in the shared import-export drill stays deferred.
