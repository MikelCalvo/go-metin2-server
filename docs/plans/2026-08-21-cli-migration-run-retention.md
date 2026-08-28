# CLI Migration-Run Retention Printer — 2026-08-21

## Objective

Add a read-only `metin2-migrate migration-run-retention` command so operators can turn a retained `GET /local/build-info` / `metin2-migrate version` JSON snapshot into the concrete shell steps that create and populate a lab `/var/metin2/migration-runs/YYYYMMDDTHHMMSSZ-<commit12>/` tree from the migration apply runbook — without opening a database, writing retention files themselves, or embedding DSNs.

`docs/workflow/lab-deployment-topology.md` already freezes the retention naming. `docs/workflow/migration-apply-runbook.md` and `docs/workflow/lab-stale-lock-recovery.md` already freeze the command order and aside-lock artifact names. This slice closes the offline printer gap called out after confirmation-gated `apply-lock-aside`.

## Contract frozen by this slice

```bash
metin2-migrate migration-run-retention \
  --build-info <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--migration-runs-base /var/metin2/migration-runs] \
  [--target-version latest] \
  [--lock-file migration-apply.lock]
```

Rules:

1. `--build-info` is required. `-` reads stdin; any other value opens a regular non-symlink file.
2. Input is capped at 64 KiB, must be valid UTF-8, non-empty after trim, not literal JSON `null`, and must decode with `DisallowUnknownFields` plus no trailing JSON into the metadata-only build-info shape (`version`, `commit`, `build_date`).
3. `commit` must be non-empty after trim. The printed tree suffix uses the first 12 characters of that trimmed commit (or the whole value when shorter), matching the lab topology `<commit12>` rule.
4. `--ops-base-url` defaults to `http://127.0.0.1:6060` and must be an absolute `http`/`https` URL with a host and no query/fragment.
5. `--migration-runs-base` defaults to `/var/metin2/migration-runs` and must be an absolute cleaned path.
6. `--target-version` defaults to `latest`. Empty/whitespace fails closed.
7. `--lock-file` defaults to `migration-apply.lock`. Empty/whitespace fails closed. The printed script treats it as a path relative to `$RUN` unless the operator supplies an absolute path.
8. On success, stdout is a plain-text shell script that:
   - sets `OPS`, `RUNS_BASE`, `TARGET_VERSION`, `LOCK_FILE`, and `COMMIT12` from the validated inputs;
   - creates `$RUNS_BASE/<UTC compact timestamp>-$COMMIT12` as `$RUN`;
   - prints metadata-safe retention redirects for catalog, ledger-snapshot(-status), plan-artifact(-status), apply-preflight(-status), apply, apply-audit-status, post-apply status, and optional daemon curls (`/local/build-info`, `/local/db/migrations/status`);
   - prints leftover-lock triage into `$RUN` only when `"$RUN/$LOCK_FILE"` still exists after apply (`apply-lock-status` live; `apply-lock-aside` as a commented / echoed operator-run hint with `--i-confirm-lab-aside-rename`), so successful apply under `set -eu` does not fail closed on an expected missing lock;
   - requires operator-exported `$DRIVER` / `$DSN` shell variables for DB-touching commands and never embeds a concrete DSN value;
   - never executes HTTP, never writes files, never opens a database, never prints executable SQL.
9. On contract failure, exit `1` with a short stderr reason and **no** stdout script.
10. Missing/unknown flags / unexpected args → usage exit `2`.

## What this is not yet

- automatic migration apply / rollback execution by the CLI printer
- automatic stale-lock expiry or `rm` helpers
- DB-engine selection or driver bundling
- embedding or logging concrete DSNs
- ground-item restart durability or SQL import/backfill
- remote admin auth
- systemd / multi-host orchestration samples

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful printer for a valid build-info snapshot includes `COMMIT12`, `$RUN` mkdir, catalog/plan/preflight/apply retention redirects, lock-status/aside retention, and daemon build-info / migrations status curls
- blank / missing commit → exit `1`, no stdout
- malformed / invalid UTF-8 / oversized / symlink build-info → exit `1`
- relative migration-runs-base or invalid ops-base-url → exit `1`
- missing flags / unexpected args → exit `2`
- usage text lists `migration-run-retention`
- stdout omits SQL / concrete DSN markers and does not claim to perform apply itself

Validation for this slice:

- `go test ./internal/migratecli -run 'MigrationRunRetention|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal / `rm` deferred.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
4. ~~Optional rollback-direction retention printer variant remains deferred; operators can still pass an explicit `--target-version` plus manual `--allow-rollback` when executing the printed apply block.~~ Done: see [CLI migration rollback-run retention](2026-08-21-cli-migration-rollback-run-retention.md).
5. ~~Hermetic `/bin/sh` execution of the printed forward/rollback retention script against build-tagged SQLite, plus soft-fail leftover-lock triage under `set -eu`.~~ Done: see [hermetic migration-run-retention SQLite apply](2026-08-28-hermetic-migration-run-retention-sqlite-apply.md). ~~Hermetic intermediate-target retention (empty→`7`, tip→`8`).~~ Done: see [hermetic migration-run-retention intermediate-target SQLite](2026-08-28-hermetic-migration-run-retention-intermediate-target-sqlite.md).
