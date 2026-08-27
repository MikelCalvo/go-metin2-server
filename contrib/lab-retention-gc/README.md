# Lab retention / GC print-only unit samples

Tree-owned, **disabled-by-default** fragments matching
[`docs/workflow/lab-retention-gc-unit-samples.md`](../../docs/workflow/lab-retention-gc-unit-samples.md).

These files exist so FreeBSD / systemd lab hosts can copy reviewable `.sample`
units and the print-only helper without inventing packaging that enables timers,
pipes printed scripts into a shell, or auto-runs aside-rename / `rm`.

## Install (manual, review first)

```bash
install -d -m 0750 /var/metin2/ops-prints
install -d -m 0750 /usr/local/libexec
install -m 0750 contrib/lab-retention-gc/metin2-print-retention-gc.sh \
  /usr/local/libexec/metin2-print-retention-gc.sh

# systemd (do NOT systemctl enable --now until reviewed)
install -m 0644 contrib/lab-retention-gc/systemd/*.sample /etc/systemd/system/
# then rename without .sample only after review, e.g.:
#   cp /etc/systemd/system/metin2-artifact-retention-gc-print.service.sample \
#      /etc/systemd/system/metin2-artifact-retention-gc-print.service

# Optional: METIN2_RUNTIME_CONFIG for backup-restore-drill printing
install -d -m 0750 /etc/metin2
install -m 0640 contrib/lab-retention-gc/env/metin2-runtime-config.env.sample \
  /etc/metin2/metin2-runtime-config.env.sample
# review, edit the retained JSON path, then:
#   cp /etc/metin2/metin2-runtime-config.env.sample /etc/metin2/metin2-runtime-config.env
install -d -m 0755 /etc/systemd/system/metin2-artifact-retention-gc-print.service.d
install -m 0644 \
  contrib/lab-retention-gc/systemd/metin2-artifact-retention-gc-print.service.d/runtime-config.conf.sample \
  /etc/systemd/system/metin2-artifact-retention-gc-print.service.d/runtime-config.conf.sample
# rename without .sample only after review

# FreeBSD / cron.d style (optional Linux-style companion; FreeBSD hosts prefer periodic)
install -m 0644 \
  contrib/lab-retention-gc/cron.d/metin2-artifact-retention-gc-print.sample \
  /etc/cron.d/metin2-artifact-retention-gc-print.sample

# FreeBSD periodic(8) weekly (preferred on FreeBSD lab hosts; stays NO until reviewed)
install -d -m 0755 /usr/local/etc/periodic/weekly
install -m 0755 \
  contrib/lab-retention-gc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample \
  /usr/local/etc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample
# review, then rename without .sample only after flipping enable deliberately:
#   cp /usr/local/etc/periodic/weekly/900.metin2-artifact-retention-gc-print.sample \
#      /usr/local/etc/periodic/weekly/900.metin2-artifact-retention-gc-print
install -m 0644 \
  contrib/lab-retention-gc/periodic/periodic.conf.sample \
  /usr/local/etc/periodic.conf.sample
# review, keep weekly_metin2_artifact_retention_gc_print_enable="NO", then merge
# reviewed knobs into /etc/periodic.conf or /usr/local/etc/periodic.conf
```

## What the helper prints

Always:

- `build-info.json`
- `artifact-retention-gc-backups.sh`
- `artifact-retention-gc-migration-runs.sh`
- `artifact-retention-gc-exports.sh` (triage for `/var/metin2/exports`)
- `migration-run-retention.sh`
- `export-quarantine-drill.sh`
- `notes.md`

Optional (env-gated):

- `backup-restore-drill.sh` when `METIN2_RUNTIME_CONFIG` points at an existing
  non-symlink regular retained runtime-config JSON snapshot. The helper never
  live-fetches ops JSON and never shells the printed scripts. Tree-owned samples:
  `env/metin2-runtime-config.env.sample` plus the systemd
  `EnvironmentFile=` drop-in under
  `systemd/metin2-artifact-retention-gc-print.service.d/`.
- `import-export-drill.sh` when `METIN2_IMPORT_EXPORT_TREE` points at an existing
  absolute non-symlink retained export/quarantine directory **and**
  `METIN2_IMPORT_DRIVER` is a non-empty opaque `database/sql` driver name.
  Optional `METIN2_IMPORT_DSN_ENV` defaults to `METIN2_IMPORT_DSN` and is
  forwarded only as the `--dsn-env` name (never a DSN value). The helper always
  passes `--i-confirm-print-sql-import-drill` when printing and still never
  opens a database or executes `import-export`.
- `artifact-gc-aside-purge-backups.sh`,
  `artifact-gc-aside-purge-migration-runs.sh`, and
  `artifact-gc-aside-purge-exports.sh` when
  `METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=YES` (case-insensitive). Optional
  `METIN2_GC_ASIDE_MIN_AGE_DAYS` defaults to `7`; optional `METIN2_GC_ASIDE_NOW`
  forwards `--now` for deterministic review. The helper always passes
  `--i-confirm-lab-gc-aside-purge` when printing and still never executes the
  printed purge scripts.
- `migration-run-retention.sh`, `export-quarantine-drill.sh`, and (when printed)
  `backup-restore-drill.sh` forward `--gamed-log-path` / `--authd-log-path` from
  `METIN2_GAMED_LOG_PATH` / `METIN2_AUTHD_LOG_PATH` (defaults
  `/var/log/metin2/gamed.log` and `/var/log/metin2/authd.log`) so the printed
  retain scripts can optionally copy daemon JSON logs when present.

## Hard rules

1. Units / cron / `periodic` may only invoke the print helper / `metin2-migrate`
   printers.
2. Never pipe printer stdout into `/bin/sh`, `bash`, `csh`, or `zsh` from the unit.
3. Never `ExecStart` / cron-run / periodic-run `rm`, `rmdir`, `unlink`,
   `find -delete`, or aside-rename of retention trees from these samples.
4. Never embed DSNs, passwords, login keys, or executable SQL.
5. Ops listeners stay loopback-only; these samples do not change bind policy.
6. FreeBSD `periodic` stays gated on
   `weekly_metin2_artifact_retention_gc_print_enable` defaulting to `"NO"`.

## What this is not

- packaging that installs **enabled** timers / cron / `periodic` entries by default
- automatic execution of printed triage / backup / apply / purge scripts
- automatic / scheduled `rm` of aged aside-renamed retention trees (print-only
  `artifact-gc-aside-purge-*.sh` under the YES gate remains review-only)
- FreeBSD port / `pkg` enable defaults
- flipping `weekly_metin2_artifact_retention_gc_print_enable` to `YES` by default
- SQL import/backfill execution or remote admin (print-only
  `import-export-drill.sh` under the env gate remains review-only)
