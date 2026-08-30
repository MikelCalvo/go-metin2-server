# Seeded Item Instance-Attributes Import-Export Drill Tip Sync — 2026-08-31

## Objective

Close the remaining operator-facing and hermetic-drill gaps after
`0027_character_item_instance_attributes` landed: seed presence-aware tip-`0003`
inventory/equipment instance attributes (including explicit all-zero / type-zero)
in the shared retained-tree `import-export-drill` SQLite proof, and tip-sync the
seeded-tree / migration-contract / attributes follow-up docs that still treat
attribute columns as unexercised by the hermetic PATH + tip-order proof.

No upsert policy, stock production driver, remote admin, tip-`0010`/`0015`
attribute SQL companions, or README churn.

## Why now

- `feat(db): add tip-0003 item instance attributes SQL companion` already ships
  additive `0027` schema, export/quarantine/import, and package-level SQLite
  harness coverage for presence-aware attributes.
- Durable FileStore rematerialize for carried / ground / safebox attributes is
  already owned on `lane/items`.
- The seeded hermetic `import-export-drill` tree still materializes
  attribute-less inventory/equipment rows (`has_attributes = 0`), so the shared
  PATH + tip-order proof never exercises real FK-linked instance-attribute
  columns after catalog tip `27`.
- Loopback character-item-state ops docs already name `has_attributes` /
  `attr0..6`; this slice closes the remaining seeded-drill contradiction called
  out as follow-up #1 on the `0027` plan.

Those contradictions are production-ops hazards for export/quarantine/import
runbooks after FileStore already rematerializes presence-aware attributes
(including explicit all-zero / type-zero).

## Contract frozen by this slice

1. Seeded hermetic retained tree exports one inventory row with authoritative
   all-zero / type-zero attributes (`has_attributes=true`, all attr types/values
   `0`) and one equipment row with authoritative non-zero attributes
   (`has_attributes=true`, `attr0_type=1` / `attr0_value=25`, `attr1_type=4` /
   `attr1_value=-5`) via `ExportCharacterItemState`, keeping the already-owned
   tip-`0003`+`0024` socket seed beside those rows.
2. Seeded `import-result.json` / status markers keep
   `"inventory_item_count": 1` (counts unchanged; attributes are additive
   columns).
3. Seeded SQL assertions require `has_attributes = 1` plus the focused attribute
   values on both `character_inventory_items` and `character_equipment_items`.
4. Empty-payload hermetic proof stays attribute-omitted / empty for tip-`0003`.
5. Upsert / stock production driver remain deferred. tip-`0010` / tip-`0015`
   attribute SQL companions stay deferred until operators choose those additive
   companions after durable FileStore rematerialize (already owned).

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live inventory/equipment repositories
- tip-`0010` / tip-`0015` attribute SQL companions
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/plans/2026-08-30-character-item-instance-attributes-sql-additive.md`
- `docs/debugging-and-profiling.md` (pointer only if still needed)
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- seeded hermetic drill proves non-empty tip-`0003`+`0024`+`0027` instance-attribute
  SQL import through the printed script
- empty hermetic proof remains green
- upsert / stock driver / tip-`0010`/`0015` attribute companions remain explicitly
  deferred

## Anti-goals / ordering constraints

- Do not invent upsert / merge policy.
- Do not register a production driver in stock binaries.
- Do not auto-run printed import commands from CLI / contrib / cron.
- Do not push `origin/main`; push only `origin/lane/items`.
