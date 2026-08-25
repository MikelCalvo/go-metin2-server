# Contrib Export-Quarantine Drill Print Helper — 2026-08-25

## Objective

Fold the already-shipped `metin2-migrate export-quarantine-drill` printer into
the tree-owned print-only helper under `contrib/lab-retention-gc/`, and print
age-based `artifact-retention-gc` triage for `/var/metin2/exports` beside the
existing backups / migration-runs GC scripts — without live `curl`, shelling
printed scripts, automatic aside-rename/`rm`, SQL import, or packaging that
enables timers by default.

## Why now

- [CLI export → offline quarantine drill printer](2026-08-25-cli-export-quarantine-drill-printer.md)
  and the hermetic `/bin/sh` HTTP proof explicitly deferred “folding
  `export-quarantine-drill` into `contrib/lab-retention-gc` print-only samples”.
- The helper already owns `migration-run-retention` (always) and
  `backup-restore-drill` (env-gated). Export/quarantine retention trees under
  `/var/metin2/exports` are the remaining tip-`0015` migration-window companion
  missing from scheduled ops-print dumps.
- `artifact-retention-gc` already accepts any absolute retention root with the
  same `YYYYMMDDTHHMMSSZ-<commit12>/` naming; backups and migration-runs are
  printed today, but exports are not, so operators invent host-local wrappers.
- Automatic GC execution, `rm` of aside trees, SQL import, and remote admin
  stay deferred.

## Contract frozen by this slice

1. `contrib/lab-retention-gc/metin2-print-retention-gc.sh` always prints:
   - `export-quarantine-drill.sh` from `$OUT/build-info.json` with
     `--gamed-log-path` / `--authd-log-path` forwarded like the other companions
   - `artifact-retention-gc-exports.sh` for `--retention-base /var/metin2/exports`
     with the same `--keep-days` as backups / migration-runs
2. The helper still never invokes `curl`, never pipes printer stdout into a
   shell, never embeds DSN/SQL, and never aside-renames or `rm`s retention
   trees (only the existing `mktemp` trap).
3. Contrib README + `docs/workflow/lab-retention-gc-unit-samples.md` inventory
   mark `export-quarantine-drill.sh` and `artifact-retention-gc-exports.sh` as
   always owned by the helper; manual companion snippets include
   `export-quarantine-drill`.
4. Prior export-quarantine plans mark the deferred contrib-fold follow-up done.
5. Focused `internal/migratecli` contrib coverage fail-closes if those markers
   regress, and hermetic helper execution proves both new scripts are printed
   with default and custom daemon log paths.

## What this is not yet

- live `curl` of ops JSON from the scheduled helper / unit
- automatic execution of printed export / backup / apply / GC scripts
- `rm` of `.gc-aside-*` trees
- FreeBSD port / `pkg` that installs or enables units by default
- SQL import/backfill from quarantined exports
- remote admin authentication
- README churn beyond operator docs already required for this helper

## Likely files to change

- `contrib/lab-retention-gc/metin2-print-retention-gc.sh`
- `contrib/lab-retention-gc/README.md`
- `internal/migratecli/contrib_lab_retention_gc_test.go`
- `docs/workflow/lab-retention-gc-unit-samples.md`
- `docs/workflow/lab-deployment-topology.md` (brief pointer if inventory lists GC bases)
- `docs/development.md` (helper inventory beside artifact-retention-gc)
- `docs/plans/2026-08-25-cli-export-quarantine-drill-printer.md`
- `docs/plans/2026-08-25-hermetic-export-quarantine-drill-http-execution-proof.md`
- this plan

## TDD and validation

Focused coverage in `internal/migratecli`:

- static sample scan expects helper / README / workflow markers for
  `export-quarantine-drill`, `artifact-retention-gc-exports.sh`, and
  `/var/metin2/exports`
- hermetic helper execution with stub `metin2-migrate` proves
  `export-quarantine-drill.sh` is always printed with default log paths
- custom `METIN2_GAMED_LOG_PATH` / `METIN2_AUTHD_LOG_PATH` are honored in the
  export-quarantine-drill argv capture
- `artifact-retention-gc-exports.sh` is always present beside the other GC
  scripts

Validation for this slice:

- `go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Exit criteria

- helper always prints export-quarantine-drill + exports GC triage
- focused contrib tests green
- prior deferred follow-ups marked done
- docs name the new always-on companion scripts

## Anti-goals / ordering constraints

- Do not auto-run printed scripts from the helper / unit / cron / periodic.
- Do not live-fetch ops JSON.
- Do not add SQL import/backfill.
- Do not push `origin/main`; push only `origin/lane/persistence`.
