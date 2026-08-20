# CLI Backup/Restore Drill Printer — 2026-08-19

## Objective

Add a read-only `metin2-migrate backup-restore-drill` command so operators can turn a retained `GET /local/runtime-config` JSON snapshot into the concrete shell steps from the file-store backup/restore runbook, without starting `gamed`, opening a database, or performing backup/restore themselves.

The loopback drill endpoints and `docs/workflow/file-store-backup-restore-drill.md` already exist. This slice closes the offline/runbook gap called out after the combined drill docs: print path-aware curl and aside-rename commands from the active persistence layout.

## Contract frozen by this slice

```bash
metin2-migrate backup-restore-drill \
  --runtime-config <path|-> \
  [--ops-base-url http://127.0.0.1:6060] \
  [--backup-base /var/metin2/backups/drill]
```

Behavior:

1. `--runtime-config` is required. `-` reads stdin; any other value opens a regular non-symlink file.
2. Input is capped at 64 KiB, must be valid UTF-8, non-empty after trim, not literal JSON `null`, and must decode with `DisallowUnknownFields` plus no trailing JSON.
3. The snapshot must expose non-empty trimmed `persistence` paths for:
   - `account_store_dir`
   - `login_ticket_store_dir`
   - `item_template_store_path`
   - `interaction_store_path`
   - `static_actor_store_path`
   - `quest_state_store_path`
4. File-path stores must use distinct cleaned parent directories. Shared parents fail closed because restore empties `filepath.Dir(snapshotPath)`.
5. Directory stores must not equal or nest under each other (lexical cleaned paths).
6. `--ops-base-url` defaults to `http://127.0.0.1:6060` and must be an absolute `http`/`https` URL with a host and no query/fragment.
7. `--backup-base` defaults to `/var/metin2/backups/drill` and must be an absolute cleaned path.
8. On success, stdout is a plain-text shell script that:
   - sets `OPS`, `BASE`, and store path variables from the snapshot;
   - prints preflight curls for `/healthz`, `/local/runtime-config`, and `/local/persistence/status`;
   - prints backup + backup/validate curls in the runbook order;
   - prints aside-rename / recreate empty destination commands for directory and file-path stores;
   - prints restore curls in the runbook order;
   - prints the post-restore `/local/persistence/status` check;
   - never executes HTTP, never writes files, never opens a database.
9. On contract failure, exit `1` with a short stderr reason and **no** stdout script.
10. Missing/unknown flags → usage exit `2`.

## What this is not yet

- automatic backup/restore execution
- daemon startup auto-restore
- ground-item / ground-gold restart durability
- repository seams or DB backfill
- remote admin auth
- shell script execution by the CLI itself

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful printer for a valid dedicated-parent layout
- shared file-store parent → exit `1`, no stdout
- blank persistence path → exit `1`
- malformed / invalid UTF-8 / oversized runtime-config → exit `1`
- missing flags / unexpected args → exit `2`
- usage text lists `backup-restore-drill`
- stdout omits SQL / DSN markers and does not claim to perform restore

Validation:

- `go test ./internal/migratecli -run 'BackupRestoreDrill|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Extract the first narrow repository seam only after offline quarantine + loopback quarantine both prove the export boundary.
2. Keep ground-item restart durability deferred until a real world-state repository exists.
3. Add deployment topology docs once production hosts are known.
4. ~~Include the optional per-store validate + crash-temp cleanup triage in the printed drill script.~~ Done: see [CLI Backup/Restore Drill Crash-Temp Preflight](2026-08-20-cli-backup-restore-drill-crash-temp-preflight.md).
