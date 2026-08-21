# Migration Apply Lock Holder Build Match — 2026-08-21

## Objective

Extend the read-only `metin2-migrate apply-lock-status` helper so operators can tell whether a retained apply lock's stamped build identity matches the inspecting `metin2-migrate` binary, without authorizing lock deletion, opening a database target, or inventing automatic stale-lock recovery.

Lock artifacts already stamp `build_version` / `build_commit` / `build_date`, and `apply-lock-status` already reports advisory local PID liveness plus hostname locality. This slice closes the remaining inspecting-binary correlation gap called out after the hostname-locality work.

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
  "holder_build_check": "local_buildinfo_current"
}
```

Field rules:

1. `holder_build_check` is the fixed probe name `local_buildinfo_current` for comparing lock stamped identity to `buildinfo.Current()` on the inspecting binary.
2. `holder_build_matches` is `true` only when trimmed `lock.build_version`, `lock.build_commit`, and `lock.build_date` each equal the corresponding trimmed `buildinfo.Current()` fields exactly; otherwise `false`.
3. Empty inspecting-binary identity fields after trim fail closed with a non-zero CLI exit and no stdout JSON, so operators do not treat an inconclusive probe as authorization to delete the lock.
4. A `false` / `true` result is advisory binary-identity evidence only. Unstamped `dev`/`none`/`unknown` defaults, rebuilt binaries with identical stamps, and copied lock files remain operator judgment problems; this helper still does **not** delete locks.
5. Existing `holder_pid_*` and `holder_hostname_*` behavior is unchanged.

## What this is not yet

- automatic lock expiry or force-unlock
- database advisory locks
- daemon-local mutation endpoints
- treating `holder_build_matches=false`, `holder_hostname_local=false`, or `holder_pid_alive=false` as permission to delete the lock
- ground-item restart durability or SQL import/backfill

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing lock still returns `present: false` without holder fields and without opening a DB;
- lock whose build identity matches `buildinfo.Current()` returns `holder_build_matches: true` and `holder_build_check: local_buildinfo_current`;
- lock whose build identity differs returns `holder_build_matches: false`;
- status still leaves the lock file in place and never prints DSN/SQL text;
- existing PID-liveness / hostname-locality / malformed / symlink / empty-host coverage remains green.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyLockStatus' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal deferred until a deployment-specific recovery policy exists.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
