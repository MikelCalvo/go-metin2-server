# Seeded Item-Template Refine Fields Import-Export Drill Tip Sync — 2026-08-30

## Objective

Close the remaining operator-facing and hermetic-drill gaps after
`0021_item_template_refine_keep_on_fail` and
`0022_item_template_refine_fail_result_vnum` landed: seed non-empty tip-`0009`
refine rows that exercise additive `keep_on_fail` and `fail_result_vnum` in the
shared retained-tree `import-export-drill` SQLite proof, and tip-sync loopback
item-template-state docs that still omit those additive refine columns.

No upsert policy, stock production driver, remote admin, tip retip to `21`/`22`,
or README churn.

## Why now

- Package SQLite harness coverage already proves tip-`0009`+`0021`+`0022` INSERT
  for both keep-on-fail and fail-result-vnum refine infos.
- The seeded hermetic `import-export-drill` tree still materializes a single
  socket-less potion template with empty `refine_infos` / `refine_materials`, so
  the shared PATH + tip-order proof never exercises real FK-linked refine columns
  after catalog tips `21`/`22`.
- `docs/debugging-and-profiling.md` item-template-state GET/quarantine still
  describe refine child rows without naming additive `keep_on_fail` /
  `fail_result_vnum`, contradicting the additive schema already owned by export
  and SQL import.

Those contradictions are production-ops hazards for export/quarantine/import
runbooks after FileStore already rematerializes authored refine keep/downgrade
fields.

## Contract frozen by this slice

1. Seeded hermetic retained tree exports four templates:
   - potion `27001` (material carrier, no refine info)
   - downgrade blade `11199` (fail-result target, no refine info)
   - wooden sword `11200` with `keep_on_fail=true`, probability `75`, one
     material `27001 x2`
   - downgrade source blade `11300` with `fail_result_vnum=11199`, probability
     `60`, one material `27001 x1`
2. Seeded `import-result.json` / status markers expect
   `"refine_info_count": 2` (counts prove refine rows; template count rises to 4).
3. Seeded SQL assertions require `COUNT(*) = 4` on `item_templates`,
   `COUNT(*) = 2` on `item_template_refine_infos` /
   `item_template_refine_materials`, plus focused `SELECT`s of the keep-on-fail
   and fail-result-vnum rows (and one keep-on-fail material).
4. Empty-payload hermetic proof stays refine-omitted / empty for tip-`0009`.
5. Operator loopback docs for item-template-state name additive
   `keep_on_fail` / `fail_result_vnum` beside tip-`0009` refine child rows.
6. Upsert / stock production driver / tip retip / safebox cell sockets /
   tip-`0010` ground SQL sockets remain deferred.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live item-template repositories
- retipping item-template exports to `migration_version=21` or `22`
- safebox cell sockets / tip-`0010` ground-item socket SQL companion
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/debugging-and-profiling.md`
- `docs/development.md`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/plans/2026-08-29-item-template-refine-fail-result-vnum-migration.md`
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- seeded hermetic drill proves non-empty tip-`0009`+`0021`/`0022` refine SQL
  import through the printed script
- loopback item-template-state docs mention additive `keep_on_fail` /
  `fail_result_vnum`
- empty hermetic proof remains green
- upsert / stock driver / tip retip remain explicitly deferred

## Anti-goals / ordering constraints

- Do not invent upsert / merge policy.
- Do not register a production driver in stock binaries.
- Do not auto-run printed import commands from CLI / contrib / cron.
- Do not push `origin/main`; push only `origin/lane/persistence`.
