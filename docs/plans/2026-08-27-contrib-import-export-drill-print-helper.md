# Contrib Import-Export Drill Print Helper — 2026-08-27

## Objective

Fold the already-shipped confirmation-gated `metin2-migrate import-export-drill`
printer into the tree-owned print-only helper under `contrib/lab-retention-gc/`,
env-gated beside `backup-restore-drill`, so operators can review a retained
export/quarantine tree → SQL import script dump under `/var/metin2/ops-prints/`
without live `curl`, shelling printed scripts, embedding a DSN value, registering
a stock production driver, inventing upsert policy, or auto-running imports.

## Why now

- [CLI import-export drill printer](2026-08-27-cli-import-export-drill.md)
  explicitly deferred “folding `import-export-drill` into
  `contrib/lab-retention-gc` print-only samples”.
- The helper already owns always-on `export-quarantine-drill` /
  `migration-run-retention` / exports GC triage and env-gated
  `backup-restore-drill`. SQL import is the remaining tip-kind companion after
  the nine import seams + CLI wiring + drill printer landed.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work
  (auto-run, stock driver registration, upsert, remote admin, DSN embedding).

## Contract frozen by this slice

1. `contrib/lab-retention-gc/metin2-print-retention-gc.sh` optionally prints
   `import-export-drill.sh` when **both**:
   - `METIN2_IMPORT_EXPORT_TREE` points at an existing absolute, non-symlink
     directory (the retained `YYYYMMDDTHHMMSSZ-<commit12>` export tree), and
   - `METIN2_IMPORT_DRIVER` is a non-empty opaque `database/sql` driver name
     literal forwarded to `--driver`.
2. Optional `METIN2_IMPORT_DSN_ENV` defaults to `METIN2_IMPORT_DSN` and is
   forwarded only as the `--dsn-env` **name**. The helper never reads or embeds
   a DSN value, password, or connection string.
3. The helper always passes `--i-confirm-print-sql-import-drill` when printing.
   Incomplete / relative / symlink / non-directory configs skip with a
   `notes.md` reason and do **not** create `import-export-drill.sh`.
4. The helper still never invokes `curl`, never pipes printer stdout into a
   shell, never opens a database, never executes `import-export`, and never
   aside-renames or `rm`s retention trees (only the existing `mktemp` trap).
5. Contrib README + `docs/workflow/lab-retention-gc-unit-samples.md` inventory
   mark `import-export-drill.sh` as env-gated beside `backup-restore-drill.sh`.
6. Prior import-export-drill plan marks the deferred contrib-fold follow-up done.
7. Focused `internal/migratecli` contrib coverage fail-closes if those markers
   regress, and hermetic helper execution proves skip / print / symlink /
   incomplete-config / custom `--dsn-env` behavior.

## What this is not yet

- automatic / scheduled execution of the printed import script
- ~~hermetic `/bin/sh` execution proof against a real driver-backed database~~ Done — see [hermetic import-export drill SQLite execution proof](2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md)
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- folding `artifact-gc-aside-purge` into scheduled print helpers
- loopback ops mutation endpoint
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `contrib/lab-retention-gc/metin2-print-retention-gc.sh`
- `contrib/lab-retention-gc/README.md`
- `contrib/lab-retention-gc/env/metin2-runtime-config.env.sample`
- `contrib/lab-retention-gc/periodic/periodic.conf.sample`
- `internal/migratecli/contrib_lab_retention_gc_test.go`
- `docs/workflow/lab-retention-gc-unit-samples.md`
- `docs/workflow/lab-deployment-topology.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/development.md`
- `docs/plans/2026-08-27-cli-import-export-drill.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- static sample scan expects helper / README / workflow markers for
  `import-export-drill`, `METIN2_IMPORT_EXPORT_TREE`, `METIN2_IMPORT_DRIVER`,
  `METIN2_IMPORT_DSN_ENV`, and `--i-confirm-print-sql-import-drill`
- hermetic helper execution proves skip without import env
- hermetic print when absolute tree directory + driver are set
- symlink / relative / incomplete configs skip
- custom `METIN2_IMPORT_DSN_ENV` appears in stub argv; no DSN value embedding

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1
gofmt -l internal/migratecli/contrib_lab_retention_gc_test.go
git diff --check
```

## Exit criteria

- helper optionally prints `import-export-drill.sh` under the env gate
- focused contrib tests green
- prior deferred follow-up marked done
- docs name the new env-gated companion script
- auto-run / stock driver / upsert / DSN embedding remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed import commands from the helper / unit / cron / periodic.
- Do not embed DSN values in helper stdout, notes, or env samples.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
