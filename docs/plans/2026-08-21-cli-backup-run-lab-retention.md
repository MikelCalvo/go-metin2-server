# CLI Backup-Restore Drill Lab Retention — 2026-08-21

## Objective

Align the read-only `metin2-migrate backup-restore-drill` printer with the lab backup retention tree already frozen in `docs/workflow/lab-deployment-topology.md`, so operators get a path-aware script that creates `/var/metin2/backups/YYYYMMDDTHHMMSSZ-<commit12>/` and retains runtime-config / persistence-status evidence beside the store backups — without executing backup/restore, opening a database, or embedding DSNs.

The existing drill printer still defaults to `/var/metin2/backups/drill`, appends a local-time `-${TS}` suffix, uses older subdirectory names (`account`, `interactions`), and does not retain `runtime-config.json` / `persistence-status-*.json`. This slice closes that retention gap after `migration-run-retention`.

## Contract frozen by this slice

```bash
metin2-migrate backup-restore-drill \
  --runtime-config <path|-> \
  --build-info <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--backup-base /var/metin2/backups]
```

Rules:

1. `--runtime-config` remains required with the existing 64 KiB / UTF-8 / strict JSON / dedicated-parent validation rules.
2. `--build-info` is now required. `-` reads stdin; any other value opens a regular non-symlink file. Input is capped at 64 KiB, must be valid UTF-8, non-empty after trim, not literal JSON `null`, and must decode with `DisallowUnknownFields` plus no trailing JSON into the metadata-only build-info shape (`version`, `commit`, `build_date`).
3. `commit` must be non-empty after trim. The printed tree suffix uses the first 12 characters of that trimmed commit (or the whole value when shorter), matching the lab topology `<commit12>` rule.
4. `--ops-base-url` defaults to `http://127.0.0.1:6060` and must be an absolute `http`/`https` URL with a host and no query/fragment.
5. `--backup-base` defaults to `/var/metin2/backups` (no longer `/var/metin2/backups/drill`) and must be an absolute cleaned path.
6. On success, stdout is a plain-text shell script that:
   - sets `OPS`, `BACKUPS_BASE`, `COMMIT12`, and the six validated store path variables;
   - creates `$BACKUPS_BASE/<UTC compact timestamp>-$COMMIT12` as `$BASE` where the timestamp is `YYYYMMDDTHHMMSSZ`;
   - retains `runtime-config.json`, `persistence-status-before.json`, and `persistence-status-after.json` under `$BASE`;
   - prepares lab-named store destinations:
     - `accounts/`
     - `login-tickets/`
     - `item-templates/`
     - `interaction-store/`
     - `static-actors/`
     - `quest-state/`
   - prints the existing aggregate preflight, store validate / crash-temp triage, backup, backup/validate, aside-rename, restore, and post-restore sequence against those destinations;
   - never executes HTTP, never writes files, never opens a database, never prints executable SQL / concrete DSNs.
7. When both `--runtime-config -` and `--build-info -` would compete for stdin, fail closed with a short stderr reason (operators must supply at least one as a regular file).
8. On contract failure, exit `1` with a short stderr reason and **no** stdout script.
9. Missing/unknown flags / unexpected args → usage exit `2`.

## What this is not yet

- automatic backup/restore execution by the CLI
- daemon startup auto-restore
- ground-item / ground-gold restart durability
- SQL import/backfill or DB-backed repository implementation
- remote admin auth
- automatic artifact GC / lifecycle jobs
- changing the loopback backup/restore endpoint contracts themselves

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful printer for valid runtime-config + build-info includes `COMMIT12`, UTC `$BASE` mkdir under `/var/metin2/backups`, lab subdirectory names, retained runtime-config / persistence-status redirects, and the existing triage → backup → restore ordering
- blank / missing commit → exit `1`, no stdout
- missing `--build-info` → usage exit `2`
- relative backup-base or invalid ops-base-url → exit `1`
- dual stdin (`--runtime-config -` and `--build-info -`) → exit `1`
- existing shared-parent / blank persistence / malformed runtime-config rejections remain green
- usage text lists `--build-info` and the `/var/metin2/backups` default
- stdout omits SQL / concrete DSN markers and does not claim to perform restore

Validation for this slice:

- `go test ./internal/migratecli -run 'BackupRestoreDrill|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
2. Keep automatic artifact GC / lifecycle jobs deferred.
3. Optional rollback-direction migration retention printer remains deferred; operators can still pass an explicit `--target-version` plus manual `--allow-rollback` when executing the printed apply block.
4. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
