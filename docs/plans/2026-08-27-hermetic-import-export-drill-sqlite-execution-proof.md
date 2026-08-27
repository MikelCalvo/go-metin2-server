# Hermetic Import-Export Drill SQLite Execution Proof — 2026-08-27

## Objective

Prove the already-shipped confirmation-gated `metin2-migrate import-export-drill`
printer emits a portable `/bin/sh` script that actually drives retained
`quarantine.json` → confirmation-gated `import-export` for every tip kind against
a build-tagged SQLite database — without inventing automatic CLI execution,
registering a stock production driver, inventing upsert policy, or exposing a
daemon mutation route.

The printer, CLI `import-export`, contrib print helper, and per-package
`Import*` SQLite harnesses already landed. Operators still lacked a hermetic
end-to-end `/bin/sh` proof that the printed PATH wiring, DSN-env indirection,
quarantine existence checks, and insert-only imports succeed together on a real
`database/sql` engine.

## Why now

- [CLI import-export drill printer](2026-08-27-cli-import-export-drill.md)
  explicitly deferred “hermetic `/bin/sh` execution proof against a real
  driver-backed database”.
- [Contrib import-export drill print helper](2026-08-27-contrib-import-export-drill-print-helper.md)
  also names that hermetic execution proof as not-yet.
- Printer stdout-shape tests and fake-driver empty-export CLI coverage do not
  exercise the combined printed script against a real migrate binary + SQLite.
- The PvE durable-state / migration-window vertical needs confidence that the
  operator runbook script can import all nine tip kinds after schema apply.

## Contract frozen by this slice

1. A build-tagged blank import under `cmd/metin2-migrate` (`//go:build
   sqlite_harness`) links pure-Go SQLite (`modernc.org/sqlite`) only when
   operators deliberately build with `-tags=sqlite_harness`. Stock
   `go build ./cmd/metin2-migrate` stays free of that driver.
2. Focused `internal/migratecli` coverage under the same build tag:
   - builds `./cmd/metin2-migrate` with `-tags=sqlite_harness` into a temp
     `PATH` directory;
   - materializes a retained export tree with empty but shape-valid
     `quarantine.json` for every `exportQuarantineKinds` entry (same empty
     payloads already owned by untagged `import-export` CLI tests);
   - applies the embedded catalog to tip on a temp SQLite file via
     `metin2-migrate empty-ledger-snapshot` + `metin2-migrate apply
     --driver sqlite --dsn <file> --ledger-snapshot <empty> --target-version
     latest`;
   - prints `import-export-drill` with `--driver sqlite`, absolute
     `--export-tree`, and `--i-confirm-print-sql-import-drill`;
   - executes the printed script under `/bin/sh` with `PATH` including the
     tagged binary and `METIN2_IMPORT_DSN` set to the temp DB DSN.
3. After execution every tip kind subdirectory contains
   `import-result.json` with the expected empty-count metadata already owned by
   each `Import*` seam (for example `"account_count": 0`,
   `"inventory_item_count": 0`, …).
4. Printed script and import-result bodies omit concrete DSN embedding beyond
   the env-var indirection already owned by the printer (no `postgres://`, no
   pasted `file:/...` DSN literals inside printer stdout).
5. Untagged `go test ./internal/migratecli` stays free of the SQLite dependency.
6. Docs mark the deferred hermetic `/bin/sh` follow-up done on the printer /
   contrib plans and Track E / migration-contract tips.

## What this is not yet

- automatic / scheduled execution of the printed import script from CLI /
  contrib / cron / periodic
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- ~~folding `artifact-gc-aside-purge` into scheduled print helpers~~ Done — see
  [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md)
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing
- non-empty seeded import payloads in this hermetic drill (empty tip-kind
  payloads remain the portable gate; seeded non-empty coverage stays owned by
  per-package `SQLiteHarness*Import` tests)

## Likely files to change

- `cmd/metin2-migrate/sqlite_harness.go` (new; build-tagged blank import)
- `internal/migratecli/import_export_drill_sqlite_harness_test.go` (new;
  build-tagged)
- `docs/plans/2026-08-27-cli-import-export-drill.md`
- `docs/plans/2026-08-27-contrib-import-export-drill-print-helper.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- `docs/development.md`
- `docs/workflow/migration-apply-runbook.md`
- this plan

## TDD and validation

Focused coverage under `//go:build sqlite_harness`:

- hermetic tagged binary + printed-script `/bin/sh` execution
- all nine tip kinds write `import-result.json` with empty-count metadata
- apply reaches catalog tip before imports
- printer stdout never embeds a concrete DSN value

Validation for this slice:

```bash
go test -tags=sqlite_harness ./internal/migratecli -run 'ImportExportDrillSQLite' -count=1
gofmt -l cmd/metin2-migrate/sqlite_harness.go internal/migratecli/import_export_drill_sqlite_harness_test.go
git diff --check
```

Also keep untagged package green:

```bash
go test ./internal/migratecli -run 'ImportExportDrill|ContribLabRetentionGC' -count=1
```

## Exit criteria

- hermetic printed-script SQLite execution proof is green under
  `-tags=sqlite_harness`
- prior deferred hermetic `/bin/sh` follow-ups mark this proof done
- stock binaries remain free of a registered production driver
- upsert / auto-run remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed import commands from CLI / contrib / cron.
- Do not embed DSN values in printer stdout or contrib notes.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
