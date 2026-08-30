# Seeded Safebox Cell Instance-Sockets Import-Export Drill Tip Sync — 2026-08-30

## Objective

Close the remaining operator-facing and hermetic-drill gaps after
`0025_character_safebox_item_instance_sockets` landed: seed presence-aware
tip-`0015` safebox cell instance sockets (including explicit zero and/or
authoritative non-zero) in the shared retained-tree `import-export-drill`
SQLite proof, and tip-sync loopback character-safebox-state docs that previously
omitted `has_sockets` / `socket0..2`.

No upsert policy, stock production driver, remote admin, tip-`0010` ground SQL
companion, or README churn.

## Why now

- `feat(db): persist safebox cell instance sockets on tip-0015 SQL import`
  already ships additive `0025` schema, export/quarantine/import, and
  package-level SQLite harness coverage for presence-aware safebox cell sockets.
- The seeded hermetic `import-export-drill` tree still materializes a
  socket-less safebox cell (`has_sockets = 0`), so the shared PATH + tip-order
  proof never exercises real FK-linked safebox instance-socket columns after
  catalog tip `25`.
- `docs/debugging-and-profiling.md` character-safebox-state GET/quarantine
  previously described item rows as if they only exposed id/cell/vnum/count/lock,
  contradicting the additive `0025` contract already owned by export.

## Contract frozen by this slice

1. Seeded hermetic retained tree exports one safebox cell with authoritative
   non-zero sockets (`has_sockets=true`, `socket0=1` / `socket1=0` /
   `socket2=7`) via `ExportCharacterSafeboxState` (item id `3001`).
2. Seeded `import-result.json` / status markers keep
   `"password_count": 1` / item count unchanged (sockets are additive columns).
3. Seeded SQL assertions require `has_sockets = 1` plus the focused socket
   values on `character_safebox_items` id `3001`.
4. Empty-payload hermetic proof stays socket-omitted / empty for tip-`0015`.
5. Operator loopback docs for character-safebox-state name presence-aware
   `has_sockets` / `socket0..2` beside the older row fields.
6. Upsert / stock production driver / tip-`0010` ground SQL sockets remain
   deferred.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live safebox repositories
- additive tip-`0010` ground-item socket SQL companion
- mall / TMP4 SAFEBOX_MONEY / attributes-on-instance
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/debugging-and-profiling.md`
- `docs/development.md` / workflow tip-sync notes as needed
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/plans/2026-08-30-safebox-cell-instance-sockets-sql-additive.md`
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Status

GREEN on `lane/items`: seeded hermetic import-export-drill proves tip-`0015`+`0025`
presence-aware safebox cell sockets through the printed PATH + tip-order SQLite
path, and operator loopback character-safebox-state docs name `has_sockets` /
`socket0..2`. Upsert / stock driver / tip-`0010` ground SQL sockets remain deferred.
