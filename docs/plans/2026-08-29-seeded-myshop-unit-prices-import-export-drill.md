# Seeded MyShop Unit-Prices Import-Export Drill Tip Sync — 2026-08-29

## Objective

Close the remaining operator-facing and hermetic-drill gaps after
`0023_character_myshop_unit_prices` landed: seed non-empty tip-`0023` rows in the
shared retained-tree `import-export-drill` SQLite proof, and tip-sync operator
docs / CLI kind inventories that still omitted `character-myshop-unit-prices`.

No upsert policy, stock production driver, remote admin, or README churn.

## Why now

- `feat(db): tip catalog with character myshop unit-prices migration` already
  ships schema, export/quarantine/import, loopback ops, and CLI kind wiring.
- The seeded hermetic `import-export-drill` tree still materializes an empty
  `character-myshop-unit-prices` payload (`price_row_count: 0`), so the shared
  PATH + tip-order proof never exercised real FK-linked price rows.
- `docs/development.md`, `docs/debugging-and-profiling.md`,
  `docs/workflow/lab-deployment-topology.md`,
  `docs/workflow/migration-apply-runbook.md`, and
  `docs/plans/2026-08-19-cli-export-quarantine.md` still described the tip kind
  vocabulary as if it stopped before `0023`.

Those contradictions are production-ops hazards for export/quarantine/import
runbooks after silk-bag unit prices already rematerialize through FileStore.

## Contract frozen by this slice

1. Seeded hermetic retained tree exports two canonical price rows for character
   `11` (`vnum=27001/unit_price=500`, `vnum=27002/unit_price=200`) via
   `ExportCharacterMyShopUnitPrices`.
2. Seeded `import-result.json` / status markers expect `"price_row_count": 2`.
3. Seeded SQL assertions require `COUNT(*) = 2` on
   `character_myshop_unit_prices` plus a focused `SELECT` of the lowest-vnum
   row (`27001` / `500`).
4. Empty-payload hermetic proof stays empty for tip-`0023`.
5. Operator docs list `character-myshop-unit-prices` beside the other tip kinds
   and document the loopback GET/POST pair.
6. Upsert / stock production driver / GD `MYSHOP_PRICELIST_*` remain deferred.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live myshop repository / runtime loading
- remote admin / daemon mutation route / secrets in git
- inventory/equipment instance-socket SQL companion (owned separately on items)

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/development.md`
- `docs/debugging-and-profiling.md`
- `docs/workflow/lab-deployment-topology.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/plans/2026-08-19-cli-export-quarantine.md`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- seeded hermetic drill proves non-empty tip-`0023` SQL import through the
  printed script
- operator kind inventories / loopback docs mention
  `character-myshop-unit-prices`
- empty hermetic proof remains green
- upsert / stock driver remain explicitly deferred

## Anti-goals / ordering constraints

- Do not invent upsert / merge policy.
- Do not register a production driver in stock binaries.
- Do not auto-run printed import commands from CLI / contrib / cron.
- Do not push `origin/main`; push only `origin/lane/persistence`.
