# Item-Template Refine Fail-Result-Vnum Migration — 2026-08-29

## Objective

Close the migration/export/import gap after items-lane owned template-authored
`refine_info.fail_result_vnum`: add additive catalog migration
`0022_item_template_refine_fail_result_vnum`, project the field through tip-`0009`
export/quarantine, and fail closed before SQL INSERT when the ledger owns
tip-`0009` / additive `0021` but not additive `0022`.

## Why now

- FileStore / runtime already round-trip and honor `fail_result_vnum` for injected
  `1..99` refine rolls (`docs/plans/2026-08-29-refine-fail-result-vnum.md`).
- Migration-shaped tip-`0009` export/import still omitted the field after `0021`
  landed `keep_on_fail`, so quarantined SQL backfill silently defaulted authored
  downgrade templates to destroy-on-fail.
- Track E prefers explicit additive schema + import preflight over opaque
  driver `no such column` errors (same pattern as `0021` keep_on_fail).
- Safe, lane-scoped, one-commit, and testable without inventing upsert policy,
  retipping export identity to `22`, or registering a stock production driver.

## Contract frozen by this slice

1. Embedded catalog adds `0022_item_template_refine_fail_result_vnum` after
   `0021_item_template_refine_keep_on_fail` (catalog tip moves to `22`).
2. `up` adds `fail_result_vnum BIGINT NOT NULL DEFAULT 0` on
   `item_template_refine_infos` with CHECK that non-zero values require
   `keep_on_fail = 0`, `probability` in `1..99`, and inequality vs source/result
   vnums (mirroring store validation).
3. `down` drops the `fail_result_vnum` column.
4. Keep tip-`0009` / `item_template_refine_info` as the export / quarantine /
   import-result migration identity.
5. `ItemTemplateRefineInfoRow` carries optional `fail_result_vnum`; export and
   quarantine canonicalize the field from reconstructed templates.
6. `ImportItemTemplateState` inserts `fail_result_vnum` and requires tip-`0009`
   plus additive `0021` plus additive `0022` before any INSERT
   (`ErrItemTemplateStateImportSchemaRequired`).
7. Upsert / auto-run / stock production driver / catalysts remain explicitly
   deferred.

## What this is not yet

- retipping item-template exports to `migration_version=22`
- DB-backed live item-template loading
- remote admin / daemon mutation route / secrets in git
- catalyst / guild / peer refine surfaces

## Likely files to change

- `db/migrations/0022_item_template_refine_fail_result_vnum.{up,down}.sql`
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
- `go test ./internal/ops -run LocalMigrationStatus -count=1`
- `go test ./internal/minimal -run 'MigrationStatus|MigrationCatalog|RegisterGamedMigration' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep upsert / stock production driver deferred.
2. Keep tip-`0009` export identity until a deliberate retip is needed.
3. Keep catalysts / guild refine / peer notifications deferred.
4. ~~Seed tip-`0009`+`0021`/`0022` refine fields in the hermetic
   `import-export-drill` retained tree.~~ Done — see
   [seeded item-template refine fields tip
   sync](2026-08-30-seeded-item-template-refine-fields-import-export-drill.md).
