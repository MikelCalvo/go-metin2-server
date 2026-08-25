# CLI Artifact-Retention GC Script Execution Proof — 2026-08-22

## Objective

Prove the already-shipped `metin2-migrate artifact-retention-gc` printer emits a
portable `/bin/sh` triage script that actually aside-renames aged lab retention
trees, leaves young / non-matching children alone, and fails closed on aside
destination collisions — without inventing deletion, scheduled GC, systemd
units, DB access, or remote admin surfaces.

The printer landed in
[CLI artifact-retention GC printer](2026-08-22-cli-artifact-retention-gc-printer.md)
with stdout-shape tests only. Operators still lacked a hermetic execution proof
that the printed FreeBSD/GNU-portable epoch helper and age gate behave as the
lab topology runbook claims.

## Contract frozen by this slice

1. Focused `internal/migratecli` coverage materializes a temporary retention
   root, prints the script with pinned `--now` / `--keep-days`, and executes it
   under `/bin/sh`.
2. Aged `YYYYMMDDTHHMMSSZ-<commit12>/` directories older than `--keep-days` are
   aside-renamed to `<name>.gc-aside-<NOW_UTC>`.
3. Younger matching trees, non-directory children, non-matching names, and
   already-aside `*.gc-aside-*` directories remain untouched.
4. When the aside destination already exists, the script exits non-zero, leaves
   the aged source tree in place, and prints a stderr reason mentioning
   `artifact-retention-gc` / the aside path.
5. The printed script still never contains `rm` / `rmdir` / `unlink` /
   `find -delete`, never opens a database, and never embeds a DSN.
6. Docs mark the printer plan's missing execution-proof gap closed; automatic /
   scheduled GC deletion and systemd/unit shipping remain deferred.

## What this is not yet

- automatic / scheduled artifact GC or lifecycle daemons
- ~~`rm` / unlink of aside-renamed trees~~ Done for confirmation-gated print-only `artifact-gc-aside-purge` — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution and folding purge into `contrib/lab-retention-gc` remain deferred.
- systemd / cron unit shipping that invokes this printer
- recursive scanning below immediate retention-base children
- SQL import/backfill or DB-engine dump GC
- remote admin authentication
- durable safebox FileStore backup/restore drill expansion (items-lane)

## TDD and validation

Focused coverage in `internal/migratecli`:

- hermetic execution renames one aged tree and preserves one young matching tree
  plus non-matching residue
- pre-existing aside destination → non-zero exit, aged source preserved
- stdout of the printer still bans deletion / SQL / DSN markers (existing
  coverage kept)

Validation for this slice:

- `go test ./internal/migratecli -run 'ArtifactRetentionGC' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled artifact GC deletion deferred.
2. ~~Keep `rm` of aside-renamed trees deferred.~~ Done for the confirmation-gated print-only `artifact-gc-aside-purge` surface — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution and folding purge into `contrib/lab-retention-gc` remain deferred.
3. ~~Optional later: systemd/unit samples that only print (never auto-run) this triage script.~~ Done: see [print-only retention / GC unit samples](2026-08-23-print-only-retention-gc-unit-samples.md) and [lab retention / GC print-only unit samples](../workflow/lab-retention-gc-unit-samples.md). Automatic execution remains deferred.
4. Keep SQL import/backfill deferred until a driver-backed harness exists.
5. ~~Optional later: fold durable safebox FileStore into the combined
   backup-restore drill after the items-lane rematerialize commit lands on
   `main`.~~ Done: see [safebox file-store backup/restore drill fold-in](2026-08-23-safebox-file-store-backup-restore-drill.md).
