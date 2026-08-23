# Contrib Companion Print Retention Printers — 2026-08-23

## Objective

Fold the already-documented optional companion printers
(`migration-run-retention` and `backup-restore-drill`) into the tree-owned
print-only helper under `contrib/lab-retention-gc/` so a single scheduled dump
retains create-script reviews beside artifact-retention-gc triage — without
live `curl`, shelling printed scripts, automatic aside-rename/`rm`, SQL import,
or packaging that enables timers by default.

## Why now

- Print-only GC unit samples and contrib `.sample` fragments already landed.
- The workflow doc still describes companion printers as an operator-local
  helper extension; reconnect/restart and apply-window labs therefore invent
  divergent one-offs.
- `migration-run-retention` needs only the build-info snapshot the helper
  already writes, so it can always print without depending on `gamed`.
- `backup-restore-drill` needs retained `/local/runtime-config` JSON; the safe
  bootstrap is an optional absolute file via `METIN2_RUNTIME_CONFIG`, never a
  scheduled live loopback `curl` from the unit.
- Automatic GC execution, `rm` of aside trees, SQL import, and remote admin
  stay deferred.

## Contract frozen by this slice

1. `contrib/lab-retention-gc/metin2-print-retention-gc.sh` always prints
   `migration-run-retention.sh` from `$OUT/build-info.json`.
2. When `METIN2_RUNTIME_CONFIG` is set to an existing non-symlink regular file,
   the helper also prints `backup-restore-drill.sh` from that retained snapshot
   plus `$OUT/build-info.json`; otherwise it skips the drill printer and notes
   the skip (missing / symlink / unset) in `notes.md`.
3. The helper still never invokes `curl`, never pipes printer stdout into a
   shell, never embeds DSN/SQL, and never aside-renames or `rm`s retention
   trees (only the existing `mktemp` trap).
4. Workflow / contrib README review-directory inventory marks
   `migration-run-retention.sh` as owned by the helper and
   `backup-restore-drill.sh` as env-gated; the ad-hoc companion snippet points
   at the helper instead of inventing a second path.
5. Focused `internal/migratecli` contrib coverage fail-closes if those markers
   regress.

## What this is not yet

- live `curl` of `/local/runtime-config` from the scheduled helper / unit
- automatic execution of printed backup / apply / GC scripts
- `rm` of `.gc-aside-*` trees
- FreeBSD port / `pkg` that installs or enables units by default
- SQL import/backfill or driver-backed harness
- remote admin authentication
- README churn beyond operator docs already required

## TDD and validation

- `go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. Keep `rm` of aside-renamed trees deferred.
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
4. Keep FreeBSD port / `pkg` enable defaults deferred.
5. Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`
   (printer remains read-only; scheduled helper still prefers retained JSON).
