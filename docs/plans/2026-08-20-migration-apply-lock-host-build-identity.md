# Migration Apply Lock Host / Build Identity — 2026-08-20

## Objective

Extend the CLI-only `go-metin2-migration-apply-lock-v1` artifact with local hostname and stamped binary build identity so operators triaging leftover locks can tell which host and which `metin2-migrate` binary reserved the window, without authorizing lock deletion, opening a database target, or inventing automatic stale-lock recovery.

Lab topology and release identity are already frozen (`docs/workflow/lab-deployment-topology.md`, `internal/buildinfo`), and `apply-lock-status` already reports advisory local PID liveness. This slice closes the remaining host/binary correlation gap called out after the death-floor restart and lock-liveness work.

## Contract frozen by this slice

`metin2-migrate apply --lock-file <path>` still creates the lock only after plan/preflight validation and before opening the database. Successful reserved locks additionally include:

```json
{
  "format": "go-metin2-migration-apply-lock-v1",
  "created_at": "...",
  "pid": 1234,
  "hostname": "lab-host",
  "build_version": "dev",
  "build_commit": "none",
  "build_date": "unknown",
  "driver": "...",
  "dsn_configured": true,
  "target_version": 1,
  "target_latest": false,
  "plan_sha256": "...",
  "ledger_snapshot_sha256": "..."
}
```

Field rules:

1. `hostname` is the local `os.Hostname()` value at reservation time, trimmed; empty or lookup failure fails closed before DB open and does not leave a partial lock.
2. `build_version` / `build_commit` / `build_date` are the metadata-only `buildinfo.Current()` fields already exposed by `metin2-migrate version` and `/local/build-info`.
3. `apply-lock-status` continues to return the strict lock object unchanged for present locks, including the new fields, plus advisory `holder_pid_alive` / `holder_pid_check`.
4. Normalize/status decoding requires non-empty trimmed hostname and build identity strings; unknown fields, DSN values, and executable SQL remain rejected.
5. Hostname / build identity are triage evidence only. They do **not** authorize automatic lock deletion, cross-host reclaim, or distributed locking.

## What this is not yet

- automatic stale-lock expiry or force-unlock
- database advisory locks
- daemon-local mutation endpoints
- treating mismatched hostname / dead PID as permission to delete the lock
- ground-item restart durability or SQL import/backfill

## TDD and validation

Focused coverage in `internal/migratecli`:

- reserved apply lock includes hostname + buildinfo fields before DB open and still excludes DSN/SQL;
- `apply-lock-status` round-trips those fields for a present lock;
- empty hostname / empty build identity fail closed on normalize/status;
- existing missing-lock / absent-PID / malformed / symlink coverage remains green.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(WritesMetadataOnlyLockFileBeforeOpeningDatabase|RejectsExistingLockFile|RemovesLockFile)|TestRunApplyLockStatus' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal deferred until a deployment-specific recovery policy exists.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. ~~Add advisory hostname locality on `apply-lock-status`.~~ Done: see [migration apply lock holder hostname locality](2026-08-21-migration-apply-lock-holder-hostname-locality.md).
4. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
