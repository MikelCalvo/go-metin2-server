# Contrib Artifact GC-Aside Purge Print Helper — 2026-08-27

## Objective

Fold the already-shipped confirmation-gated `metin2-migrate artifact-gc-aside-purge`
printer into the tree-owned print-only helper under `contrib/lab-retention-gc/`,
env-gated beside `backup-restore-drill` / `import-export-drill`, so operators can
review aged `.gc-aside-*` purge scripts under `/var/metin2/ops-prints/` without
the helper / unit / cron / periodic executing `rm`, aside-renaming live trees,
opening a database, embedding a DSN, or enabling scheduled purge by default.

## Why now

- [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md)
  explicitly deferred “folding `artifact-gc-aside-purge` into
  `contrib/lab-retention-gc` print-only samples”.
- [Contrib import-export drill print helper](2026-08-27-contrib-import-export-drill-print-helper.md)
  and hermetic import-export drill proof also keep that purge fold deferred.
- The helper already owns always-on `artifact-retention-gc` triage for backups /
  migration-runs / exports. The second half of retention triage (print-only
  aged aside purge) is the remaining companion that still forces host-local
  wrappers.
- Safe, lane-scoped, one-commit, and testable without deferred dangerous work
  (auto-run of `rm`, scheduled purge enable-by-default, stock DB driver,
  upsert, remote admin).

## Contract frozen by this slice

1. `contrib/lab-retention-gc/metin2-print-retention-gc.sh` optionally prints
   three purge companion scripts when
   `METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE` matches `YES` (case-insensitive):
   - `artifact-gc-aside-purge-backups.sh` for `/var/metin2/backups`
   - `artifact-gc-aside-purge-migration-runs.sh` for `/var/metin2/migration-runs`
   - `artifact-gc-aside-purge-exports.sh` for `/var/metin2/exports`
2. Optional `METIN2_GC_ASIDE_MIN_AGE_DAYS` defaults to `7` and is forwarded to
   `--min-aside-age-days`. Blank uses the printer default. Non-integer / `< 1`
   values skip with a `notes.md` reason and do **not** create purge scripts.
3. Optional `METIN2_GC_ASIDE_NOW` is forwarded to `--now` only when non-empty
   (RFC3339 / compact UTC). Operators normally omit it so the printer uses host
   UTC wall clock.
4. The helper always passes `--i-confirm-lab-gc-aside-purge` when printing.
   Missing / non-`YES` gate values skip with a `notes.md` reason and do not
   create purge scripts.
5. The helper still never invokes `curl`, never pipes printer stdout into a
   shell, never opens a database, never executes the printed purge scripts, and
   never aside-renames or `rm`s retention trees (only the existing `mktemp`
   trap). Helper source must not contain the literal `.gc-aside-` marker or
   live `rm` of retention paths.
6. Contrib README + `docs/workflow/lab-retention-gc-unit-samples.md` inventory
   mark the three purge scripts as env-gated beside `import-export-drill.sh`.
7. Prior artifact-gc-aside-purge / import-export-drill / Track E tips mark the
   deferred contrib-fold follow-up done; automatic / scheduled *execution* of
   printed purge scripts remains explicitly deferred.
8. Focused `internal/migratecli` contrib coverage fail-closes if those markers
   regress, and hermetic helper execution proves skip / print / invalid-age /
   custom min-age / custom `--now` behavior.

## What this is not yet

- automatic / scheduled execution of the printed purge scripts
- flipping any timer / cron / `periodic` enable flag to YES by default
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- FreeBSD port / `pkg` enable defaults
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
- `docs/development.md`
- `docs/plans/2026-08-25-cli-artifact-gc-aside-purge-printer.md`
- `docs/plans/2026-08-27-contrib-import-export-drill-print-helper.md`
- `docs/plans/2026-08-27-cli-import-export-drill.md`
- `docs/plans/2026-08-27-hermetic-import-export-drill-sqlite-execution-proof.md`
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- static sample scan expects helper / README / workflow / env / periodic markers
  for `artifact-gc-aside-purge`, `METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE`,
  `METIN2_GC_ASIDE_MIN_AGE_DAYS`, `METIN2_GC_ASIDE_NOW`, and
  `--i-confirm-lab-gc-aside-purge`
- hermetic helper execution proves skip without the YES gate
- hermetic print of all three purge scripts when gate is YES
- invalid min-age skips
- custom min-age / custom `--now` appear in stub argv
- helper source and units still forbid live shell-of-printer / retention `rm`

Validation for this slice:

```bash
go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1
gofmt -l internal/migratecli/contrib_lab_retention_gc_test.go
git diff --check
```

## Exit criteria

- helper optionally prints the three purge companion scripts under the YES gate
- focused contrib tests green
- prior deferred contrib-fold follow-ups marked done
- docs name the new env-gated companion scripts
- auto-run / enable-by-default / stock driver / upsert remain explicitly deferred

## Anti-goals / ordering constraints

- Do not auto-run printed purge commands from the helper / unit / cron / periodic.
- Do not put the literal `.gc-aside-` marker into helper source (keep the
  existing print-only forbidden-marker contract).
- Do not flip periodic / timer enable defaults to YES.
- Do not register a production driver in stock binaries.
- Do not invent upsert / merge policy.
- Do not push `origin/main`; push only `origin/lane/persistence`.
