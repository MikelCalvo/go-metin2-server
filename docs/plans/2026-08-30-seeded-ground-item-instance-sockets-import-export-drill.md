# Seeded Ground-Item Instance-Sockets Import-Export Drill Tip Sync — 2026-08-30

## Objective

Close the remaining hermetic-drill gap after
`0026_bootstrap_ground_item_instance_sockets` landed: seed presence-aware
tip-`0010` pending ground instance sockets (authoritative non-zero) in the
shared retained-tree `import-export-drill` SQLite proof, and tip-sync the
seeded-tree / Track C plans that previously left ground sockets as an optional
follow-up.

No upsert policy, stock production driver, remote admin, `ITEM_GROUND_ADD` wire
sockets, or README churn.

## Why now

- `feat(db): add tip-0010 ground-item instance sockets SQL companion` already
  ships additive `0026` schema, export/quarantine/import, and package-level
  SQLite harness coverage for presence-aware pending ground sockets.
- The seeded hermetic `import-export-drill` tree still materializes a
  socket-less ground row (`has_sockets = 0`), so the shared PATH + tip-order
  proof never exercises real FK-linked ground instance-socket columns after
  catalog tip `26`.
- Loopback bootstrap-ground-item-state docs already name presence-aware
  `has_sockets` / `socket0..2`; this slice freezes the hermetic drill twin
  beside the already-owned tip-`0003`+`0024` and tip-`0015`+`0025` seed proofs.

## Contract frozen by this slice

1. Seeded hermetic retained tree exports one item-shaped ground handle with
   authoritative non-zero sockets (`has_sockets=true`, `socket0=1` /
   `socket1=0` / `socket2=7`) via `ExportBootstrapGroundItemState`
   (VID `0x0700002c`, vnum `3001`).
2. Seeded `import-result.json` / status markers keep
   `"ground_item_count": 1` unchanged (sockets are additive columns).
3. Seeded SQL assertions require `has_sockets = 1` plus the focused socket
   values on `bootstrap_ground_items`.
4. Empty-payload hermetic proof stays socket-omitted / empty for tip-`0010`.
5. Gold-shaped rows remain socket-less (unchanged).
6. Upsert / stock production driver / DB-backed live ground rematerialize /
   `ITEM_GROUND_ADD` wire sockets remain deferred.

## What this is not yet

- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed live ground repositories replacing FileStore rematerialize
- changing `GC::ITEM_GROUND_ADD` / ownership wire layouts
- remote admin / daemon mutation route / secrets in git

## Likely files to change

- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-seeded-tree.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/plans/2026-08-30-bootstrap-ground-item-instance-sockets-sql-additive.md`
- `docs/plans/2026-08-30-seeded-safebox-cell-instance-sockets-import-export-drill.md`
- this plan

## TDD and validation

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
go test ./internal/migratecli -run 'ImportExportDrill|QuarantineExport|ExportQuarantineDrill' -count=1
gofmt -l internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Status

GREEN on `lane/items`: seeded hermetic import-export-drill proves tip-`0010`+`0026`
presence-aware pending ground sockets through the printed PATH + tip-order SQLite
path. Operator docs tip-sync after catalog tip `0026` is owned by
[ops docs tip sync after catalog tip 0026](2026-08-30-ops-docs-0026-ground-sockets-tip-sync.md).
Upsert / stock driver / live DB ground rematerialize / wire sockets remain
deferred.
