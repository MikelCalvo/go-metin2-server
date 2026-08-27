# Contrib FreeBSD Periodic Retention / GC Print Sample — 2026-08-23

## Objective

Ship a **disabled-by-default** FreeBSD `periodic(8)` weekly fragment under
`contrib/lab-retention-gc/` that only invokes the already-owned print helper —
without enabling `periodic` by default, piping printed triage scripts into a
shell, live-fetching ops JSON, or inventing a FreeBSD port / `pkg` with
`ENABLE` defaults.

## Why now

- Lab topology and retention/GC unit samples already own systemd + Linux-style
  `/etc/cron.d` fragments, but this host (and the documented FreeBSD lab path)
  actually schedules maintenance through `periodic weekly`.
- Recent Track E follow-ups still list "FreeBSD port / `pkg` enable defaults"
  as deferred. This slice is narrower: a reviewable `periodic` `.sample`
  fragment operators copy by hand, still defaulting to `NO`.
- Automatic GC execution, `rm` of aside trees, SQL import, and remote admin stay
  deferred.

## Contract frozen by this slice

1. New tree fragments under `contrib/lab-retention-gc/`:
   - `periodic/weekly/900.metin2-artifact-retention-gc-print.sample`
   - `periodic/periodic.conf.sample`
2. The weekly script:
   - sources `/etc/defaults/periodic.conf` + `source_periodic_confs` when present;
   - gates on `weekly_metin2_artifact_retention_gc_print_enable` matching `YES`
     (case-insensitive); default remains unset / `NO`;
   - when enabled, runs only
     `/usr/local/libexec/metin2-print-retention-gc.sh` (or
     `METIN2_PRINT_RETENTION_GC_HELPER` override) and records the helper exit;
   - never pipes helper / printer stdout into `/bin/sh`, never `rm`s retention
     trees, never embeds DSN / SQL / password markers, never `curl`s ops JSON.
3. `periodic/periodic.conf.sample` documents only:
   - `weekly_metin2_artifact_retention_gc_print_enable="NO"`
   - optional commented `METIN2_*` overrides that do not embed secrets.
4. Focused `internal/migratecli` coverage fail-closes if the periodic sample is
   missing or regresses the hard rules.
5. Workflow / contrib README point at the FreeBSD periodic install path; the
   existing `/etc/cron.d` sample remains as an optional Linux-style companion.
6. Automatic enablement (`YES` default), FreeBSD port / `pkg` enable defaults,
   and automatic execution of printed triage scripts remain deferred.

## What this is not yet

- FreeBSD port / `pkg` that installs or enables periodic / cron by default
- flipping the sample `*_enable` default to `YES`
- automatic execution of printed aside-rename / backup / apply scripts
- `rm` of `.gc-aside-*` trees
- live scheduled `curl` of `/local/runtime-config`
- SQL import/backfill or remote admin

## TDD and validation

- `go test ./internal/migratecli -run 'ContribLabRetentionGC' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic / scheduled execution of printed triage scripts deferred.
2. ~~Keep `rm` of aside-renamed trees deferred.~~ Done for the confirmation-gated print-only `artifact-gc-aside-purge` surface — see [CLI artifact GC-aside purge printer](2026-08-25-cli-artifact-gc-aside-purge-printer.md). Automatic / scheduled purge execution remains deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).
3. Keep SQL import/backfill deferred until a driver-backed harness exists.
4. Keep FreeBSD port / `pkg` enable defaults deferred.
5. ~~Optional later: hermetic end-to-end HTTP drill against a live drained `gamed`.~~
   Done: see
   [hermetic backup/restore drill HTTP execution proof](2026-08-24-hermetic-backup-restore-drill-http-execution-proof.md).
