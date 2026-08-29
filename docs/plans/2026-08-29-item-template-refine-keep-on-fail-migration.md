# Item-Template Refine Keep-On-Fail Migration — 2026-08-29

## Objective

Close the migration/export/import gap after items-lane owned template-authored
`refine_info.keep_on_fail`: add additive catalog migration
`0021_item_template_refine_keep_on_fail`, project the flag through tip-`0009`
export/quarantine, and fail closed before SQL INSERT when the ledger owns
tip-`0009` but not additive `0021`.

## Why now

- FileStore / runtime already round-trip and honor `keep_on_fail` for injected
  `1..99` refine rolls (`docs/plans/2026-08-29-refine-keep-on-fail.md`).
- Migration-shaped tip-`0009` export/import still omitted the flag, so
  quarantined SQL backfill silently defaulted keep-grade templates to destroy.
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as chase/return/homeward/max/
  reaction delay beside tip-`0013`).
- Safe, lane-scoped, one-commit, and testable without inventing upsert policy,
  retipping export identity to `21`, or registering a stock production driver.

## Contract frozen by this slice

1. Embedded catalog adds `0021_item_template_refine_keep_on_fail` after
   `0020_static_actor_combat_profile_reaction_delay` (catalog tip moves to `21`).
2. `up` adds `keep_on_fail INTEGER NOT NULL DEFAULT 0` on
   `item_template_refine_infos` with
   `CHECK (keep_on_fail = 0 OR (keep_on_fail = 1 AND probability >= 1 AND probability <= 99))`.
3. `down` drops the `keep_on_fail` column.
4. Keep tip-`0009` / `item_template_refine_info` as the export / quarantine /
   import-result migration identity.
5. `ItemTemplateRefineInfoRow` carries optional `keep_on_fail`; export and
   quarantine canonicalize the flag from reconstructed templates.
6. `ImportItemTemplateState` inserts `keep_on_fail` and requires tip-`0009`
   plus additive `0021` before any INSERT (`ErrItemTemplateStateImportSchemaRequired`).
7. Upsert / auto-run / stock production driver / `fail_result_vnum` SQL column
   remain explicitly deferred.

## What this is not yet

- retipping item-template exports to `migration_version=21`
- SQL column / import for deferred `fail_result_vnum`
- DB-backed live item-template loading
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `db/migrations/0021_item_template_refine_keep_on_fail.{up,down}.sql`
- `db/migrations/migrations.manifest.json`
- `db/migrations/catalog_test.go` / `plan_test.go`
- `internal/itemstore/migration_export.go`
- `internal/itemstore/migration_export_quarantine.go`
- `internal/itemstore/item_template_state_import.go`
- `internal/itemstore/*_test.go` (+ sqlite harness)
- `internal/migratecli/import_export_test.go`
- `internal/ops/pprofmux_test.go`
- `internal/minimal/factory_test.go` / `gamed_migration_ops_test.go`
- `docs/development.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- this plan

## TDD and validation

- `go test ./db/migrations -run 'BuiltInCatalog|CatalogSummaryUsesBuiltIn|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/itemstore -run 'ItemTemplateState|ExportItemTemplate|ValidateItemTemplate' -count=1`
- `go test -tags=sqlite_harness ./internal/itemstore -run SQLiteHarnessItemTemplateStateImport -count=1`
- `go test ./internal/migratecli -run ImportExport -count=1`
- `go test -tags=sqlite_harness ./internal/migratecli -run ImportExportDrillSQLite -count=1`
- `go test ./internal/ops -run LocalMigrationStatus -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep `fail_result_vnum` SQL column deferred until items-lane GREEN lands.
2. Keep upsert / stock production driver deferred.
3. Keep tip-`0009` export identity until a deliberate retip is needed.
