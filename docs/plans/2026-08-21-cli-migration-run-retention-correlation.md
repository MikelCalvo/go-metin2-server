# CLI Migration-Run Retention Correlation Checklist — 2026-08-21

## Objective

Extend the read-only `metin2-migrate migration-run-retention` printer so the generated lab `/var/metin2/migration-runs/YYYYMMDDTHHMMSSZ-<commit12>/` script retains the full operator correlation checklist already frozen in `docs/workflow/lab-deployment-topology.md` — both-daemon build-info, runtime-config, persistence status before/after mutation, and a `notes.md` stub — without opening a database, writing retention files itself, embedding DSNs, or inventing a second CLI command.

The forward and rollback printers already create the migration metadata tree and redirect catalog / ledger / plan / preflight / apply / lock-triage artifacts. Operators still hand-curl `authd` identity, runtime-config, persistence status, and operator notes. This slice closes that correlation gap after the rollback-direction retention printer.

## Contract frozen by this slice

```bash
metin2-migrate migration-run-retention \
  --build-info <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--authd-ops-base-url http://127.0.0.1:6061] \
  [--migration-runs-base /var/metin2/migration-runs] \
  [--target-version latest] \
  [--lock-file <name-or-absolute-path>] \
  [--allow-rollback]
```

Rules:

1. Existing forward and `--allow-rollback` validation remains unchanged for `--build-info`, `--ops-base-url`, `--migration-runs-base`, `--target-version`, `--lock-file`, and rollback artifact naming.
2. `--authd-ops-base-url` defaults to `http://127.0.0.1:6061` and must be an absolute `http`/`https` URL with a host and no query/fragment (same normalization as `--ops-base-url`).
3. On success, stdout still creates `$RUNS_BASE/<UTC compact timestamp>-$COMMIT12` as `$RUN` and still never executes HTTP, never writes files, never opens a database, never prints executable SQL / concrete DSNs.
4. The printed script additionally:
   - sets `AUTH_OPS` from the validated authd ops base URL;
   - retains `curl -sS \"$AUTH_OPS/local/build-info\" > \"$RUN/authd-build-info.json\"` beside the existing gamed build-info retain;
   - retains `curl -sS \"$OPS/local/runtime-config\" > \"$RUN/runtime-config.json\"`;
   - retains `curl -sS \"$OPS/local/persistence/status\" > \"$RUN/persistence-status-before.json\"` before the offline catalog / ledger / plan / preflight block;
   - retains `curl -sS \"$OPS/local/persistence/status\" > \"$RUN/persistence-status-after.json\"` in the post-apply / post-rollback retention block;
   - prints a `notes.md` stub under `$RUN` via a small shell heredoc with operator checklist placeholders only (no secrets, no DSNs, no SQL).
5. Existing daemon migrations-status curl, metadata redirects, apply/rollback mutation lines, and optional lock triage / aside-rename lines stay in place.
6. On contract failure, exit `1` with a short stderr reason and **no** stdout script.
7. Missing/unknown flags / unexpected args → usage exit `2`. Usage text lists `--authd-ops-base-url`.

## What this is not yet

- automatic migration apply / rollback execution by the CLI printer
- automatic stale-lock expiry or `rm` helpers
- automatic artifact GC / lifecycle jobs
- DB-engine selection or driver bundling
- embedding or logging concrete DSNs
- ground-item restart durability or SQL import/backfill
- remote admin auth
- systemd / multi-host orchestration samples
- treating retained correlation curls as proof that a live database still matches the retained ledger snapshot

## TDD and validation

Focused coverage in `internal/migratecli`:

- forward printer includes `AUTH_OPS`, authd build-info, runtime-config, persistence-status-before, persistence-status-after, and `notes.md` stub ordering around the existing catalog → apply → post-status flow
- rollback printer keeps rollback artifact names / `--allow-rollback` while still printing the same correlation retains
- invalid `--authd-ops-base-url` → exit `1`, no stdout
- existing blank commit / relative base / malformed / symlink / usage / rollback-latest rejection coverage remains green
- stdout omits SQL / concrete DSN markers and does not claim to perform apply/rollback itself

Validation for this slice:

- `go test ./internal/migratecli -run 'MigrationRunRetention|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal / `rm` deferred.
2. Keep automatic artifact GC / lifecycle jobs deferred.
3. Add DB-engine-specific advisory lock coverage once a production driver is selected.
4. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
5. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
