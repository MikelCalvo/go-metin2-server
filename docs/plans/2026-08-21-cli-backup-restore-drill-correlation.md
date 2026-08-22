# CLI Backup-Restore Drill Correlation Checklist — 2026-08-21

## Objective

Extend the read-only `metin2-migrate backup-restore-drill` printer so the generated lab `/var/metin2/backups/YYYYMMDDTHHMMSSZ-<commit12>/` script retains the operator correlation checklist already frozen for migration windows — both-daemon build-info (`gamed` + `authd`) and a `notes.md` stub — without opening a database, writing retention files itself, embedding DSNs, or inventing a second CLI command.

The lab topology and migration-run retention printers already require both-daemon identity plus operator notes. The backup drill printer still only retains `runtime-config.json` / `persistence-status-*.json` and has no `--authd-ops-base-url`. This slice closes that correlation gap after `migration-run-retention` correlation.

## Contract frozen by this slice

```bash
metin2-migrate backup-restore-drill \
  --runtime-config <path|-> \
  --build-info <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--authd-ops-base-url http://127.0.0.1:6061] \
  [--backup-base /var/metin2/backups]
```

Rules:

1. Existing `--runtime-config`, `--build-info`, `--ops-base-url`, and `--backup-base` validation remains unchanged (including dual-stdin rejection).
2. `--authd-ops-base-url` defaults to `http://127.0.0.1:6061` and must be an absolute `http`/`https` URL with a host and no query/fragment (same normalization as `--ops-base-url`).
3. On success, stdout still creates `$BACKUPS_BASE/<UTC compact timestamp>-$COMMIT12` as `$BASE` and still never executes HTTP, never writes files, never opens a database, never prints executable SQL / concrete DSNs.
4. The printed script additionally:
   - sets `AUTH_OPS` from the validated authd ops base URL;
   - retains `curl -sS \"$OPS/local/build-info\" > \"$BASE/gamed-build-info.json\"` before runtime-config retain;
   - retains `curl -sS \"$AUTH_OPS/local/build-info\" > \"$BASE/authd-build-info.json\"` beside gamed build-info;
   - keeps existing runtime-config / persistence-status-before retains after identity retain;
   - prints a `notes.md` stub under `$BASE` via a small shell heredoc with operator checklist placeholders only (no secrets, no DSNs, no SQL), after preflight status retain and before store validate / crash-temp triage;
   - keeps the existing validate → cleanup → backup → backup/validate → aside-rename → restore → persistence-status-after sequence unchanged.
5. On contract failure, exit `1` with a short stderr reason and **no** stdout script.
6. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists `--authd-ops-base-url`.

## What this is not yet

- automatic backup/restore execution by the CLI printer
- automatic artifact GC / lifecycle jobs
- ground-item / ground-gold restart durability
- SQL import/backfill or DB-backed repository implementation
- remote admin auth
- changing the loopback backup/restore endpoint contracts themselves
- treating retained correlation curls as proof that restored stores match the retained backup manifests

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful printer includes `AUTH_OPS`, gamed/authd build-info retains, `notes.md` stub ordering around the existing runtime-config → status-before → validate → backup flow
- invalid `--authd-ops-base-url` → exit `1`, no stdout
- custom `--authd-ops-base-url` is honored in printed `AUTH_OPS`
- usage text lists `--authd-ops-base-url`
- existing blank commit / relative base / malformed / dual-stdin / shared-parent rejection coverage remains green
- stdout omits SQL / concrete DSN markers and does not claim to perform restore

Validation for this slice:

- `go test ./internal/migratecli -run 'BackupRestoreDrill|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic artifact GC / lifecycle jobs deferred.
2. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
3. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
4. ~~Optional Docker `LABEL` workflow-run metadata remains deferred.~~ Done: see [Docker LABEL workflow-run metadata](2026-08-22-docker-label-workflow-run-metadata.md).
