# Migration Apply Lock Age — 2026-08-21

## Objective

Extend the read-only `metin2-migrate apply-lock-status` helper so operators can see how long a retained apply lock has been held, without authorizing lock deletion, opening a database target, or inventing automatic stale-lock recovery.

Lock artifacts already stamp `created_at`, and `apply-lock-status` already reports advisory local PID liveness, hostname locality, and build-identity match. This slice closes the remaining wall-clock triage gap for leftover locks after interrupted apply windows.

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
  "holder_hostname_check": "local_os_hostname",
  "holder_build_matches": true,
  "holder_build_check": "local_buildinfo_current",
  "lock_age_seconds": 3600,
  "lock_age_check": "local_wall_clock"
}
```

Field rules:

1. `lock_age_check` is the fixed probe name `local_wall_clock` for comparing parsed `lock.created_at` to the inspecting host's current UTC wall clock.
2. `lock_age_seconds` is the non-negative whole-second floor of `now.UTC().Sub(created_at)`. Fractional seconds are truncated toward zero; a lock created in the future yields `0` rather than a negative age.
3. `created_at` continues to be validated as RFC3339Nano during lock decode; age computation never invents a substitute timestamp.
4. A large or small age is advisory wall-clock evidence only. Clock skew, copied lock files, and long-running intentional applies remain operator judgment problems; this helper still does **not** delete locks and does **not** define an expiry threshold.
5. Existing `holder_pid_*`, `holder_hostname_*`, and `holder_build_*` behavior is unchanged.

## What this is not yet

- automatic lock expiry or force-unlock
- a deployment-specific stale-age threshold that authorizes deletion
- database advisory locks
- daemon-local mutation endpoints
- treating `lock_age_seconds` (alone or with `holder_pid_alive=false`) as permission to delete the lock
- ground-item restart durability or SQL import/backfill

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing lock still returns `present: false` without age fields and without opening a DB;
- lock whose `created_at` is one hour before the injected inspection clock returns `lock_age_seconds: 3600` and `lock_age_check: local_wall_clock`;
- lock whose `created_at` is slightly in the future relative to the inspection clock returns `lock_age_seconds: 0`;
- status still leaves the lock file in place and never prints DSN/SQL text;
- existing PID-liveness / hostname-locality / build-match / malformed / symlink / empty-host coverage remains green.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyLockStatus' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. ~~Keep automatic stale-lock removal deferred until a deployment-specific recovery policy exists.~~ Done for the single-host lab topology: see [lab stale-lock recovery](../workflow/lab-stale-lock-recovery.md) and the advisory `manual_clear_candidate` bit on `apply-lock-status`. Automatic deletion remains deferred.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
