# Seeded Safebox Cell Instance-Attributes Import-Export Drill Tip Sync — 2026-08-31

## Objective

Close the remaining operator-facing and hermetic-drill gaps after
`0028_character_safebox_item_instance_attributes` landed: seed presence-aware
tip-`0015` safebox cell instance attributes (including authoritative non-zero)
in the shared retained-tree `import-export-drill` SQLite proof, and tip-sync the
seeded-tree / migration-contract / attributes follow-up docs that still treat
safebox attribute columns as unexercised by the hermetic PATH + tip-order proof.

No upsert policy, stock production driver, remote admin, tip-`0010` ground
attribute SQL companion, or README churn.

## Why now

- `feat(db): add tip-0015 safebox cell instance attributes SQL companion`
  already ships additive `0028` schema, export/quarantine/import, and
  package-level SQLite harness coverage for presence-aware safebox cell
  attributes.
- Durable FileStore rematerialize for safebox cell attributes is already owned
  on `lane/items`.
- Tip-`0003`+`0027` inventory/equipment attributes already seed the shared
  hermetic drill
  (`docs/plans/2026-08-31-seeded-item-instance-attributes-import-export-drill.md`).
- The seeded hermetic `import-export-drill` tree still materializes an
  attribute-less safebox cell (`has_attributes = 0`) beside owned tip-`0015`+`0025`
  sockets, so the shared PATH + tip-order proof never exercises real FK-linked
  safebox instance-attribute columns after catalog tip `28`.
- Follow-up #1 on the `0028` GREEN plan explicitly prefers this tip sync before
  tip-`0010` ground attribute SQL.

Those contradictions are production-ops hazards for export/quarantine/import
runbooks after FileStore already rematerializes presence-aware attributes
(including explicit all-zero / type-zero).

## Contract frozen by this slice

1. Seeded hermetic retained tree exports one safebox cell with authoritative
   non-zero attributes (`has_attributes=true`, `attr0_type=1` / `attr0_value=25`,
   `attr1_type=4` / `attr1_value=-5`) via `ExportCharacterSafeboxState`, keeping
   the already-owned tip-`0015`+`0025` socket seed on the same cell (item id
   `3001`).
2. Seeded `import-result.json` / status markers keep
   `\"password_count\": 1` / item count unchanged (attributes are additive
   columns).
3. Seeded SQL assertions require `has_attributes = 1` plus the focused attribute
   values on `character_safebox_items` id `3001`.
4. Empty-payload hermetic proof stays attribute-omitted / empty for tip-`0015`.
5. Operator loopback character-safebox-state docs already name presence-aware
   `has_attributes` / `attr0..6` beside sockets; this slice does not retip export
   identity.
6. Upsert / stock production driver remain deferred. tip-`0010` ground attribute
   SQL companion stays deferred until operators choose that additive companion
   after durable FileStore rematerialize (already owned).

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live safebox repositories
- tip-`0010` ground-item attribute SQL companion
- mall / TMP4 SAFEBOX_MONEY / client `SAFEBOX_CHANGE_PASSWORD` packets
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/plans/2026-08-30-character-safebox-item-instance-attributes-sql-additive.md`
- `docs/plans/2026-08-31-safebox-cell-instance-attributes-sql-additive.md`
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Status

GREEN on `lane/items`: seeded hermetic import-export-drill proves tip-`0015`+`0025`+`0028`
presence-aware safebox cell attributes through the printed PATH + tip-order
SQLite path. Upsert / stock driver remain deferred; tip-`0010` ground attribute
SQL companion stays deferred.
