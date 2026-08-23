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

# FreeBSD / cron.d style
install -m 0644 \
  contrib/lab-retention-gc/cron.d/metin2-artifact-retention-gc-print.sample \
  /etc/cron.d/metin2-artifact-retention-gc-print.sample
```

## What the helper prints

Always:

- `build-info.json`
- `artifact-retention-gc-backups.sh`
- `artifact-retention-gc-migration-runs.sh`
- `migration-run-retention.sh`
- `notes.md`

Optional (env-gated):

- `backup-restore-drill.sh` when `METIN2_RUNTIME_CONFIG` points at an existing
  non-symlink regular retained runtime-config JSON snapshot. The helper never
  live-fetches ops JSON and never shells the printed scripts.

## Hard rules

1. Units / cron may only invoke the print helper / `metin2-migrate` printers.
2. Never pipe printer stdout into `/bin/sh`, `bash`, `csh`, or `zsh` from the unit.
3. Never `ExecStart` / cron-run `rm`, `rmdir`, `unlink`, `find -delete`, or
   aside-rename of retention trees from these samples.
4. Never embed DSNs, passwords, login keys, or executable SQL.
5. Ops listeners stay loopback-only; these samples do not change bind policy.

## What this is not

- packaging that installs **enabled** timers / cron entries by default
- automatic execution of printed triage / backup / apply scripts
- `rm` of `.gc-aside-*` trees
- FreeBSD port / `pkg` enable defaults
- SQL import/backfill or remote admin
