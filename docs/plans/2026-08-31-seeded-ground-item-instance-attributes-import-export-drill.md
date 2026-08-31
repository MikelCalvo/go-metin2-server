# Seeded Ground-Item Instance-Attributes Import-Export Drill Tip Sync — 2026-08-31

## Objective

Close the remaining hermetic-drill gap after
`0029_bootstrap_ground_item_instance_attributes` landed: seed presence-aware
tip-`0010` pending ground instance attributes (authoritative non-zero) in the
shared retained-tree `import-export-drill` SQLite proof, and tip-sync the
seeded-tree / migration-contract / Track C plans that previously left ground
attributes as an optional follow-up.

No upsert policy, stock production driver, remote admin, `ITEM_GROUND_ADD` wire
attributes, or README churn.

## Why now

- `feat(db): add tip-0010 ground-item instance attributes SQL companion` already
  ships additive `0029` schema, export/quarantine/import, and package-level
  SQLite harness coverage for presence-aware pending ground attributes.
- Durable FileStore rematerialize for pending ground attributes is already owned
  on `lane/items`.
- Tip-`0003`+`0027` inventory/equipment attributes and tip-`0015`+`0028` safebox
  cell attributes already seed the shared hermetic drill.
- The seeded hermetic `import-export-drill` tree still materializes an
  attribute-less ground row (`has_attributes = 0`) beside owned tip-`0010`+`0026`
  sockets, so the shared PATH + tip-order proof never exercises real FK-linked
  ground instance-attribute columns after catalog tip `29`.
- Follow-up tip sync on the `0029` GREEN plan explicitly prefers this drill twin
  before inventing upsert / stock-driver / wire-attribute work.

Those contradictions are production-ops hazards for export/quarantine/import
runbooks after FileStore already rematerializes presence-aware attributes
(including explicit all-zero / type-zero).

## Contract frozen by this slice

1. Seeded hermetic retained tree exports one item-shaped ground handle with
   authoritative non-zero attributes (`has_attributes=true`, `attr0_type=1` /
   `attr0_value=25`, `attr1_type=4` / `attr1_value=-5`) via
   `ExportBootstrapGroundItemState` (VID `0x0700002c`, vnum `3001`), keeping the
   already-owned tip-`0010`+`0026` socket seed on the same row.
2. Seeded `import-result.json` / status markers keep
   `"ground_item_count": 1` unchanged (attributes are additive columns).
3. Seeded SQL assertions require `has_attributes = 1` plus the focused attribute
   values on `bootstrap_ground_items`.
4. Empty-payload hermetic proof stays attribute-omitted / empty for tip-`0010`.
5. Gold-shaped rows remain attribute-less (unchanged).
6. Upsert / stock production driver / DB-backed live ground rematerialize /
   `ITEM_GROUND_ADD` wire attributes / refine catalysts / mall remain deferred.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live ground repositories replacing FileStore rematerialize
- changing `GC::ITEM_GROUND_ADD` / ownership wire layouts
- remote admin / daemon mutation route / secrets in git
- refine catalysts / mall / TMP4 SAFEBOX_MONEY

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/plans/2026-08-31-bootstrap-ground-item-instance-attributes-sql-additive.md`
- `docs/plans/2026-08-31-seeded-safebox-cell-instance-attributes-import-export-drill.md`
- `docs/debugging-and-profiling.md` (pointer only if still needed)
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Status

GREEN on `lane/items`: seeded hermetic import-export-drill proves tip-`0010`+`0026`+`0029`
presence-aware pending ground attributes through the printed PATH + tip-order
SQLite path. Upsert / stock driver / live DB ground rematerialize / wire
attributes / refine catalysts / mall remain deferred.
