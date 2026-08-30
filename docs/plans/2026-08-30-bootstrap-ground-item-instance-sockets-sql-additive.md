# Bootstrap ground-item instance-sockets SQL additive — 2026-08-30

## Objective

Close the migration/export/import gap after items-lane owned FileStore
presence-aware pending ground instance sockets: add additive catalog migration
`0026_bootstrap_ground_item_instance_sockets`, project those sockets through
tip-`0010` export/quarantine/import, and fail closed before SQL INSERT when the
ledger owns tip-`0010` but not additive `0026`.

## Why now

- Durable pending ground FileStore / runtime already round-trip and honor
  instance sockets (including explicit zero) through drop → `gamed` restart →
  pickup (`docs/plans/2026-08-29-ground-item-instance-sockets-durable.md`).
- Migration-shaped tip-`0010` export/import still omit sockets after that GREEN,
  so quarantined SQL backfill silently drops authoritative ground instance
  sockets (including deactivated auto-potion `socket0 = 0`).
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as tip-`0003`+`0024` and
  tip-`0015`+`0025`).
- Safer than inventing a new tip identity: sockets extend existing
  `bootstrap_ground_items` rows while export identity stays tip `10`.

## Contract frozen by this slice

1. Embedded catalog adds `0026_bootstrap_ground_item_instance_sockets` after
   `0025_character_safebox_item_instance_sockets` (catalog tip moves to `26`).
2. `up` adds `has_sockets` + `socket0`/`socket1`/`socket2` on
   `bootstrap_ground_items` with CHECKs mirroring `0024`/`0025`:
   - `has_sockets IN (0, 1)`
   - each socket in signed int32 range
   - when `has_sockets = 0`, all sockets must be `0`
3. `down` drops those columns (dependent `socket2` first).
4. Keep tip-`0010` / `bootstrap_ground_item_state` as the export / quarantine /
   import-result migration identity (do **not** retip to `26`).
5. `BootstrapGroundItemStateRow` / `GroundItemSnapshot` carry optional
   `has_sockets` + `socket0`/`socket1`/`socket2`; export maps:
   - `HasSockets == false` / omitted → omitted / `has_sockets=false`, sockets `0`
   - `HasSockets == true` (including all-zero) → `has_sockets=true` + values
6. Quarantine rejects non-zero sockets when `has_sockets` is false.
7. Gold-shaped rows stay socket-less (reject `has_sockets` / non-zero sockets).
8. `ImportBootstrapGroundItemState` inserts the new columns and requires
   tip-`0010` plus additive `0026` before any INSERT
   (`ErrBootstrapGroundItemStateImportSchemaRequired` when either boundary is
   missing).
9. Durable FileStore → tip-`0010` projection
   (`DurableGroundItemRecordsToSnapshots`) carries the same presence-aware
   sockets so operator export does not silently drop FileStore authority.
10. Upsert / stock production driver / DB-backed live ground rematerialize /
    remote admin / `ITEM_GROUND_ADD` wire sockets remain deferred.

## What this is not yet

- retipping ground-item-state exports to `migration_version=26`
- DB-backed live ground rematerialize (FileStore remains the restart path)
- remote admin / daemon mutation route / secrets in git
- changing `GC::ITEM_GROUND_ADD` / ownership wire layouts
- seeded hermetic import-export-drill ground socket rows (optional follow-up)

## Likely files to change (GREEN)

- `db/migrations/0026_bootstrap_ground_item_instance_sockets.{up,down}.sql`
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
- `go test ./internal/worldruntime -run 'ExportBootstrapGround|ValidateBootstrapGround|QuarantineBootstrapGround|ImportBootstrapGround|InstanceSockets' -count=1`
- `go test -tags=sqlite_harness ./internal/worldruntime -run SQLiteHarnessGroundItemStateImport -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport|ImportExport' -count=1`
- `go test ./internal/ops -run 'LocalMigrationStatus' -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Status

Contract freeze + GREEN owned by this persistence-lane run.
