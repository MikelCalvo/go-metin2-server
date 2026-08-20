# Migration Apply Lock Holder Hostname Locality — 2026-08-21

## Objective

Extend the read-only `metin2-migrate apply-lock-status` helper so operators can tell whether a retained apply lock's `hostname` matches the current host, without authorizing lock deletion, opening a database target, or inventing automatic stale-lock recovery.

Lock artifacts already stamp `hostname` plus build identity, and `apply-lock-status` already reports advisory local PID liveness. This slice closes the remaining local-host correlation gap for leftover locks copied across hosts or inspected after a hostname change.

## Contract frozen by this slice

`metin2-migrate apply-lock-status --lock-file <path>` keeps every existing fail-closed validation rule and still:

- never deletes or rewrites the lock file;
- never opens a database, executes SQL, applies/rolls back migrations, or reserves audit files;
- never emits DSNs or executable SQL;
- returns `present: false` with no holder fields when the lock path is absent.

When `present: true`, successful output additionally includes:

```json
{
  "format": "go-metin2-migration-apply-lock-status-v1",
  "present": true,
  "lock": { "...": "..." },
  "holder_pid_alive": true,
  "holder_pid_check": "local_signal_0",
  "holder_hostname_local": true,
  "holder_hostname_check": "local_os_hostname"
}
```

Field rules:

1. `holder_hostname_check` is the fixed probe name `local_os_hostname` for the current lab hosts (`os.Hostname()` compared to `lock.hostname`).
2. `holder_hostname_local` is `true` when the trimmed current hostname equals the already-normalized lock hostname exactly; otherwise `false`.
3. Hostname lookup failures fail closed with a non-zero CLI exit and no stdout JSON, so operators do not treat an inconclusive probe as authorization to delete the lock.
4. A `false` / `true` result is advisory host-table evidence only. Copied lock files, container hostnames, and DNS aliases remain operator judgment problems; this helper still does **not** delete locks.
5. Existing `holder_pid_alive` / `holder_pid_check` behavior is unchanged.

## What this is not yet

- automatic lock expiry or force-unlock
- comparing stamped build identity against the inspecting binary as a status field
- database advisory locks
- daemon-local mutation endpoints
- treating `holder_hostname_local=false` or `holder_pid_alive=false` as permission to delete the lock
- ground-item restart durability or SQL import/backfill

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing lock still returns `present: false` without holder fields and without opening a DB;
- lock whose hostname matches `os.Hostname()` returns `holder_hostname_local: true` and `holder_hostname_check: local_os_hostname`;
- lock whose hostname differs returns `holder_hostname_local: false`;
- status still leaves the lock file in place and never prints DSN/SQL text;
- existing PID-liveness / malformed / symlink / empty-host coverage remains green.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyLockStatus' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal deferred until a deployment-specific recovery policy exists.
2. Add advisory `holder_build_matches` / `holder_build_check` comparing lock build identity to `buildinfo.Current()` on the inspecting binary.
3. Add DB-engine-specific advisory lock coverage once a production driver is selected.
4. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
