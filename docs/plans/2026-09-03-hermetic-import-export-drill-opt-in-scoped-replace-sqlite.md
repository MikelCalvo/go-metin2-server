# Hermetic import-export-drill opt-in scoped-replace SQLite proof — 2026-09-03

## Objective

Prove the already-shipped confirmation-gated `metin2-migrate import-export-drill
--i-confirm-print-scoped-replace` printer emits a portable `/bin/sh` script that
actually drives retained `quarantine.json` → confirmation-gated
`import-export --i-confirm-sql-import --i-confirm-scoped-replace` against a
build-tagged SQLite database — without inventing automatic CLI execution,
registering a stock production driver, auto-enabling replace by default,
cascade-deleting child tip domains from tip-`0002`, or exposing a daemon
mutation route.

## Why now

- Tip vocabulary scoped-replace GREEN is complete for every landed export kind
  (`0002` / `0003` / `0004` / `0011` / `0015` / `0023` / `0010` / `0009` /
  `0013` / `0007`).
- Offline `import-export-status` already accepts wipe / multi-history scope
  slices — see
  [import-export-status scoped-replace identity slices](2026-09-03-import-export-status-scoped-replace-identity-slices.md).
- Print-only opt-in scoped-replace printer already landed — see
  [import-export-drill opt-in scoped-replace printer](2026-09-03-import-export-drill-opt-in-scoped-replace.md).
- Existing hermetic empty/seeded insert-only SQLite proofs still do **not**
  exercise `--i-confirm-print-scoped-replace` end-to-end against a real
  `database/sql` engine.
- Lab re-backfill after tip rebuild is the remaining Track E operator-friction
  gap before production-engine selection.
- RED against a naive parent-first replace order proved tip-`0002` roster
  scoped replace fails closed on live child FKs. RED against child-before-roster
  **seeded re-backfill** further proved that child tip replace re-inserts FK rows,
  so tip-`0002` still fails closed in the **same single pass**. That limitation
  must be frozen honestly rather than inventing cascade delete.

## Contract frozen by this slice

1. Keep the existing empty-payload and seeded hermetic **insert-only** proofs
   intact (default printer remains insert-only and keeps the current
   `exportQuarantineKinds` parent-first order starting with
   `account-character-roster`).
2. When `--i-confirm-print-scoped-replace` is set, the printer emits tip-kind
   `import-export` lines in **FK-safe scoped-replace order**:
   1. `character-item-state`
   2. `character-point-state`
   3. `character-myshop-unit-prices`
   4. `character-quest-state`
   5. `character-safebox-state`
   6. `bootstrap-ground-item-state`
   7. `auth-login-ticket-handoff`
   8. `item-template-state`
   9. `static-actor-content-state`
   10. `account-character-roster` (last)
   so character-scoped child tip deletes can run before tip-`0002` when child
   tip rows are absent (empty tree / wipe-to-empty). Still no cascade delete
   inside any `Import*` primitive.
3. Printer comments must state that a **seeded full-tree single-pass** re-backfill
   including tip-`0002` still fails closed while child tip rows remain, because
   child tip scoped replace re-inserts FK dependents before roster delete.
   Operators who need roster replace on a populated tree must wipe/omit child tip
   domains first (or run tip-`0002` alone after children are absent). Two-phase
   wipe→roster→reimport automation remains deferred.
4. Hermetic proofs under `//go:build sqlite_harness`:
   - **Empty tree:** print + `/bin/sh` execute the full FK-safe scoped-replace
     script (including roster). Every tip kind writes `import-result.json` /
     `import-result-status.json` with `"replaced": true` and empty-count markers.
   - **Seeded tree:** insert-only seed first, then print the FK-safe
     scoped-replace script, assert roster is last, execute the printed script
     with tip-`0002` `account-character-roster` import-export / status lines
     omitted (operator-omit-roster pass), assert every non-roster tip kind has
     `"replaced": true` plus seeded non-zero markers, and
     `assertSeededImportExportDrillSQLiteRows` still holds.
5. Untagged `go test ./internal/migratecli` stays free of the SQLite dependency
   and owns the FK-safe order assertion for the scoped-replace printer path.
6. Still no stock production driver, no daemon mutation route, no auto-run of
   the printed script, and no FileStore→SQL runtime repository.

## What this is not yet

- automatic / scheduled execution of the printed import script from CLI /
  contrib / cron / periodic
- production DB engine selection as a stock default / bundled release driver
- DB-backed runtime repositories replacing FileStores
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- inventing upsert / merge / truncate-all / cascade-delete policy beyond the
  already-owned scoped-replace contracts
- two-phase wipe→roster→reimport printer automation for seeded full-tree
  re-backfill including tip-`0002`

## Likely files to change

- `internal/migratecli/import_export_drill.go`
- `internal/migratecli/import_export_drill_test.go`
- `internal/migratecli/import_export_drill_sqlite_harness_test.go`
- `docs/plans/2026-09-03-import-export-drill-opt-in-scoped-replace.md`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- this plan

## TDD and validation

```bash
go test ./internal/migratecli -run 'ImportExportDrill' -count=1
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
gofmt -l internal/migratecli/import_export_drill.go internal/migratecli/import_export_drill_test.go internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

## Exit criteria

- empty-tree hermetic full scoped-replace proof is green
- seeded-tree hermetic omit-roster scoped-replace re-backfill proof is green
- FK-safe scoped-replace print order is owned by untagged printer tests
- seeded tip-`0002` single-pass limitation is documented, not papered over
- stock binaries remain free of a registered production driver
- production-engine selection / auto-run / cascade-delete remain deferred

## Anti-goals / ordering constraints

- Do not auto-enable scoped replace without `--i-confirm-print-scoped-replace`.
- Do not change insert-only `exportQuarantineKinds` order for the default
  printer path.
- Do not invent cascade delete inside tip-`0002` roster replace.
- Do not change `Import*` result shapes or quarantine contracts.
- Do not register a stock production driver.
- Do not push `origin/main`; push only `origin/lane/persistence`.
