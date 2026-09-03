# Import-export-drill opt-in scoped-replace printer — 2026-09-03

## Objective

Extend the confirmation-gated print-only `metin2-migrate import-export-drill`
surface so operators can **opt in** to printing `--i-confirm-scoped-replace` on
every tip-kind `import-export` line when re-backfilling a retained export tree,
without auto-enabling replace by default and without registering a stock
production driver.

## Why now

- Tip vocabulary scoped-replace GREEN is complete for every landed export kind
  (`0002` / `0003` / `0004` / `0011` / `0015` / `0023` / `0010` / `0009` /
  `0013` / `0007`) and offline `import-export-status` already accepts wipe /
  multi-history scope slices — see
  [import-export-status scoped-replace identity slices](2026-09-03-import-export-status-scoped-replace-identity-slices.md).
- Every prior scoped-replace freeze deferred a separate drill-printer slice
  rather than auto-enable replace.
- Lab re-backfill after tip rebuild currently requires hand-editing the printed
  insert-only drill script; that is the remaining Track E operator-friction gap
  before production-engine selection.

## Contract frozen by this slice

```bash
metin2-migrate import-export-drill \
  --export-tree /var/metin2/exports/YYYYMMDDTHHMMSSZ-<commit12> \
  --driver <database/sql-driver-name> \
  [--dsn-env METIN2_IMPORT_DSN] \
  --i-confirm-print-sql-import-drill \
  [--i-confirm-print-scoped-replace]
```

Behavior:

1. Default (no `--i-confirm-print-scoped-replace`) remains **insert-only**: each
   printed `import-export` line still has only `--i-confirm-sql-import`.
2. When `--i-confirm-print-scoped-replace` is also set, every printed
   `import-export` line additionally includes `--i-confirm-scoped-replace`
   after `--i-confirm-sql-import`.
3. The printer still never executes imports, never opens a database, never
   embeds a DSN value, and never invents upsert / truncate-all policy.
4. Opt-in remains confirmation-gated at print time; execution still requires the
   existing `import-export` confirmations.
5. Contrib print helper may forward the same opt-in only when an explicit
   `METIN2_IMPORT_PRINT_SCOPED_REPLACE=YES` gate is set (default remains
   insert-only).
6. No stock production driver, no daemon mutation route, no auto-run of the
   printed script, no FileStore→SQL runtime repository.

## Likely files to change

- `internal/migratecli/import_export_drill.go`
- `internal/migratecli/import_export_drill_test.go`
- `contrib/lab-retention-gc/metin2-print-retention-gc.sh`
- `contrib/lab-retention-gc/README.md`
- `contrib/lab-retention-gc/env/metin2-runtime-config.env.sample`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/plans/2026-08-27-cli-import-export-drill.md` (mark follow-up)
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md` when needed
- this plan

## TDD and validation

```bash
go test ./internal/migratecli -run 'ImportExportDrill' -count=1
gofmt -l internal/migratecli/import_export_drill.go internal/migratecli/import_export_drill_test.go
git diff --check
```

Focused coverage:

1. Default happy path still omits `--i-confirm-scoped-replace`.
2. Opt-in flag prints `--i-confirm-scoped-replace` on every tip-kind
   `import-export` line.
3. Usage / missing print confirmation still fail closed; opt-in alone is not
   enough without `--i-confirm-print-sql-import-drill`.

## Status

GREEN on `lane/persistence`. `metin2-migrate import-export-drill` now accepts
opt-in `--i-confirm-print-scoped-replace` (default remains insert-only). Contrib
helper forwards the same gate via `METIN2_IMPORT_PRINT_SCOPED_REPLACE=YES`.
Hermetic `/bin/sh` SQLite proof for that opt-in path is owned by
[hermetic import-export-drill opt-in scoped-replace SQLite](2026-09-03-hermetic-import-export-drill-opt-in-scoped-replace-sqlite.md)
(empty-tree full replace + seeded omit-roster re-backfill; FK-safe print order
puts tip-`0002` last; seeded tip-`0002` single-pass while child rows remain stays
fail-closed). Production-engine selection / auto-run / cascade-delete remain
deferred.

## Anti-goals

- Do not auto-enable scoped replace without the new flag.
- Do not change `Import*` result shapes or quarantine contracts.
- Do not register a stock production driver.
- Do not push `origin/main`.
