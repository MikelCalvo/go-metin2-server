# CLI Migration Rollback-Run Retention Printer — 2026-08-21

## Objective

Extend the read-only `metin2-migrate migration-run-retention` printer with an explicit `--allow-rollback` mode so operators can print the rollback-direction retention tree and command order already frozen in `docs/workflow/migration-apply-runbook.md` — without opening a database, writing retention files, embedding DSNs, or inventing a second CLI command.

The forward printer already creates `/var/metin2/migration-runs/YYYYMMDDTHHMMSSZ-<commit12>/` and redirects catalog / ledger / plan / preflight / apply artifacts. Rollback drills still require operators to hand-edit `--allow-rollback`, rollback artifact names, and `migration-rollback.lock`. This slice closes that deferred gap after the lab-aligned backup-restore-drill printer.

## Contract frozen by this slice

```bash
metin2-migrate migration-run-retention \
  --build-info <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--migration-runs-base /var/metin2/migration-runs] \
  [--target-version latest] \
  [--lock-file <name-or-absolute-path>] \
  [--allow-rollback]
```

Rules:

1. Existing forward-path validation is unchanged when `--allow-rollback` is absent:
   - `--build-info` required; 64 KiB / UTF-8 / strict metadata-only JSON; non-empty trimmed `commit`;
   - `--ops-base-url` absolute `http`/`https` with host and no query/fragment;
   - `--migration-runs-base` absolute cleaned path (default `/var/metin2/migration-runs`);
   - `--target-version` defaults to `latest`; empty/whitespace fails closed;
   - `--lock-file` defaults to `migration-apply.lock`; empty/whitespace fails closed;
   - printed script still omits `--allow-rollback` and keeps forward artifact names.
2. When `--allow-rollback` is present:
   - `--target-version` is required and must **not** be the literal `latest` after trim (operators must name an explicit rollback version such as `0`);
   - if `--lock-file` is omitted, the printed default becomes `migration-rollback.lock` instead of `migration-apply.lock`;
   - stdout still creates `$RUNS_BASE/<UTC compact timestamp>-$COMMIT12` as `$RUN`;
   - printed plan / preflight / audit / post-status redirects use the rollback runbook names:
     - `rollback-plan-artifact.json`
     - `rollback-plan-artifact-status.json`
     - `rollback-apply-preflight.json`
     - `rollback-apply-preflight-status.json`
     - `migration-rollback-audit.json`
     - `rollback-apply-audit-status.json`
     - `post-rollback-status.json`
   - catalog / ledger-snapshot(-status) / daemon curls / lock triage names stay shared with the forward printer;
   - `apply-preflight` and `apply` lines include `--allow-rollback`;
   - `apply` uses `--audit-file \"$RUN/migration-rollback-audit.json\"` and `--lock-file \"$RUN/$LOCK_FILE\"`.
3. The printer remains read-only: never executes HTTP, never writes files, never opens a database, never prints executable SQL / concrete DSNs. Operators must still export `$DRIVER` / `$DSN` before running printed DB-touching commands.
4. On contract failure, exit `1` with a short stderr reason and **no** stdout script.
5. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists `--allow-rollback`.

## What this is not yet

- automatic migration apply / rollback execution by the CLI printer
- automatic stale-lock expiry or `rm` helpers
- DB-engine selection or driver bundling
- embedding or logging concrete DSNs
- ground-item restart durability or SQL import/backfill
- remote admin auth
- systemd / multi-host orchestration samples
- treating `--allow-rollback` alone as permission to run against a live database without retained backups

## TDD and validation

Focused coverage in `internal/migratecli`:

- `--allow-rollback` with explicit `--target-version 0` prints rollback artifact names, `--allow-rollback` on preflight/apply, default `LOCK_FILE='migration-rollback.lock'`, and `COMMIT12`
- `--allow-rollback` with `--target-version latest` (or omitted default latest) → exit `1`, no stdout
- forward path without `--allow-rollback` remains unchanged (no rollback artifact names / no `--allow-rollback` lines)
- existing blank commit / relative base / malformed / symlink / usage coverage remains green
- stdout omits SQL / concrete DSN markers and does not claim to perform apply/rollback itself

Validation for this slice:

- `go test ./internal/migratecli -run 'MigrationRunRetention|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal / `rm` deferred.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
4. ~~Keep automatic artifact GC / lifecycle jobs deferred.~~ Done for the offline aside-rename printer: see [CLI artifact-retention GC printer](2026-08-22-cli-artifact-retention-gc-printer.md). Automatic / scheduled deletion remains deferred.
5. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
6. ~~Print the full operator correlation checklist (`authd` build-info, runtime-config, persistence status, `notes.md`) from the retention printer.~~ Done: see [CLI migration-run retention correlation](2026-08-21-cli-migration-run-retention-correlation.md).
7. ~~Hermetic `/bin/sh` execution of the printed forward/rollback retention script against build-tagged SQLite, plus soft-fail leftover-lock triage under `set -eu`.~~ Done: see [hermetic migration-run-retention SQLite apply](2026-08-28-hermetic-migration-run-retention-sqlite-apply.md).
