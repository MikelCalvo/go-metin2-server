# Seeded Item Instance-Sockets Import-Export Drill Tip Sync — 2026-08-30

## Objective

Close the remaining operator-facing and hermetic-drill gaps after
`0024_character_item_instance_sockets` landed: seed presence-aware tip-`0003`
inventory/equipment instance sockets (including explicit zero) in the shared
retained-tree `import-export-drill` SQLite proof, and tip-sync loopback
character-item-state docs that still omit `has_sockets` / `socket0..2`.

No upsert policy, stock production driver, remote admin, tip-`0010` ground SQL
companion, or README churn.

## Why now

- `feat(db): persist item instance sockets on tip-0003 SQL import` already ships
  additive `0024` schema, export/quarantine/import, and package-level SQLite
  harness coverage for presence-aware sockets.
- The seeded hermetic `import-export-drill` tree still materializes socket-less
  inventory/equipment rows (`has_sockets = 0`), so the shared PATH + tip-order
  proof never exercises real FK-linked instance-socket columns after catalog tip
  `24`.
- `docs/debugging-and-profiling.md` character-item-state GET/quarantine still
  describe inventory/equipment rows as if they only expose id/slot/vnum/count/
  lock, contradicting the additive `0024` contract already owned by export.

Those contradictions are production-ops hazards for export/quarantine/import
runbooks after FileStore already rematerializes presence-aware sockets
(including deactivated auto-potion `socket0 = 0`).

## Contract frozen by this slice

1. Seeded hermetic retained tree exports one inventory row with authoritative
   all-zero sockets (`has_sockets=true`, `socket0/1/2 = 0`) and one equipment
   row with authoritative non-zero sockets (`has_sockets=true`,
   `socket0=1` / `socket1=0` / `socket2=7`) via `ExportCharacterItemState`.
2. Seeded `import-result.json` / status markers keep
   `"inventory_item_count": 1` (counts unchanged; sockets are additive columns).
3. Seeded SQL assertions require `has_sockets = 1` plus the focused socket
   values on both `character_inventory_items` and `character_equipment_items`.
4. Empty-payload hermetic proof stays socket-omitted / empty for tip-`0003`.
5. Operator loopback docs for character-item-state name presence-aware
   `has_sockets` / `socket0..2` beside the older row fields.
6. Upsert / stock production driver remain deferred. tip-`0010` ground SQL
   sockets shipped as `0026` — see
   [bootstrap ground-item instance-sockets SQL additive](2026-08-30-bootstrap-ground-item-instance-sockets-sql-additive.md)
   — and seeded tip sync is owned by
   [seeded ground-item instance-sockets tip sync](2026-08-30-seeded-ground-item-instance-sockets-import-export-drill.md).
   Safebox cell sockets shipped as `0025` and seeded tip sync is owned by
   [seeded safebox cell instance-sockets tip sync](2026-08-30-seeded-safebox-cell-instance-sockets-import-export-drill.md).

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live inventory/equipment repositories
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/debugging-and-profiling.md`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/plans/2026-08-29-character-item-instance-sockets-migration.md`
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- seeded hermetic drill proves non-empty tip-`0003`+`0024` instance-socket SQL
  import through the printed script
- loopback character-item-state docs mention presence-aware sockets
- empty hermetic proof remains green
- upsert / stock driver remain explicitly deferred; tip-`0010`+`0026` ground
  socket seed tip sync is owned by
  [seeded ground-item instance-sockets tip sync](2026-08-30-seeded-ground-item-instance-sockets-import-export-drill.md)

## Anti-goals / ordering constraints

- Do not invent upsert / merge policy.
- Do not register a production driver in stock binaries.
- Do not auto-run printed import commands from CLI / contrib / cron.
- Do not push `origin/main`; push only `origin/lane/persistence`.
