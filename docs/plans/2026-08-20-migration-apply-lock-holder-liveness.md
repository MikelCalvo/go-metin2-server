# Migration Apply Lock Holder Liveness — 2026-08-20

## Objective

Extend the read-only `metin2-migrate apply-lock-status` helper so operators can tell whether the recorded local lock `pid` still appears alive on the current host, without authorizing lock deletion, opening a database target, or inventing automatic stale-lock recovery.

Lab topology and artifact retention are now frozen, so the earlier "add process ownership inspection after host policy is known" follow-up can land as a narrow status enrichment rather than a recovery policy.

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
  "holder_pid_check": "local_signal_0"
}
```

Field rules:

1. `holder_pid_check` is the fixed probe name `local_signal_0` for the current Unix lab hosts (signal `0` existence check against `lock.pid`).
2. `holder_pid_alive` is `true` when the probe indicates the PID exists locally (`kill(pid, 0)` success or `EPERM`), and `false` when the probe reports no such process (`ESRCH`).
3. Probe failures other than "not found" fail closed with a non-zero CLI exit and no stdout JSON, so operators do not treat an inconclusive probe as authorization to delete the lock.
4. A `false` / `true` result is advisory process-table evidence only. PID reuse, container PID namespaces, and cross-host leftover lock files remain operator judgment problems; this helper still does **not** delete locks.

## What this is not yet

- automatic lock expiry or force-unlock
- hostname / deployment-identity binding inside the lock file
- database advisory locks
- daemon-local mutation endpoints
- treating `holder_pid_alive=false` as permission to delete the lock
- ground-item restart durability or SQL import/backfill

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing lock still returns `present: false` without holder fields and without opening a DB;
- lock whose `pid` is `os.Getpid()` returns `holder_pid_alive: true` and `holder_pid_check: local_signal_0`;
- lock whose `pid` is a known-absent local PID returns `holder_pid_alive: false`;
- status still leaves the lock file in place and never prints DSN/SQL text;
- existing malformed/symlink rejection coverage remains green.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyLockStatus' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal deferred until a deployment-specific recovery policy exists.
2. ~~Add hostname / build-identity fields to the lock artifact.~~ Done: see [migration apply lock host / build identity](2026-08-20-migration-apply-lock-host-build-identity.md).
3. Add DB-engine-specific advisory lock coverage once a production driver is selected.
