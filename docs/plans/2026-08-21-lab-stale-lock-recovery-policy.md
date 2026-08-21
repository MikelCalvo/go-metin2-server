# Lab Stale-Lock Recovery Policy — 2026-08-21

## Objective

Freeze the first deployment-specific recovery policy for leftover `metin2-migrate apply --lock-file` artifacts on the single-host lab topology, and expose a matching advisory `manual_clear_candidate` bit on the existing read-only `apply-lock-status` helper.

Lock triage already reports PID liveness, hostname locality, build-identity match, and wall-clock age. Operators still lacked an explicit lab rule for when a leftover lock may be moved aside manually. This slice closes that runbook gap without authorizing automatic lock deletion, opening a database target, or inventing a remote unlock API.

## Contract frozen by this slice

1. `docs/workflow/lab-stale-lock-recovery.md` owns the lab recovery policy:
   - inspect first with `metin2-migrate apply-lock-status`;
   - retain `apply-lock-status.json` under the migration-runs tree;
   - never auto-delete locks from the CLI or daemons;
   - manual aside-rename is allowed only after the advisory candidate gate below and operator judgment;
   - proceed with a fresh lock path only after the aside rename and DB/file backup evidence are retained.
2. `metin2-migrate apply-lock-status --lock-file <path>` keeps every existing fail-closed validation rule and still:
   - never deletes or rewrites the lock file;
   - never opens a database, executes SQL, applies/rolls back migrations, or reserves audit files;
   - never emits DSNs or executable SQL;
   - returns `present: false` with no holder / age / candidate fields when the lock path is absent.
3. When `present: true`, successful output additionally includes:
   - `manual_clear_candidate` — advisory boolean computed from the existing probes;
   - `manual_clear_check` — fixed probe name `lab_stale_lock_policy_v1`.
4. `manual_clear_candidate` is `true` only when **all** of the following hold:
   - `holder_pid_alive == false`
   - `holder_hostname_local == true`
   - `holder_build_matches == true`
   - `lock_age_seconds >= 3600`
5. A `true` candidate is triage evidence only. It does **not** delete the lock, does **not** authorize `rm`, and does **not** replace retained preflight/audit/backup judgment. Operators must still follow the aside-rename runbook.
6. Existing `holder_pid_*`, `holder_hostname_*`, `holder_build_*`, and `lock_age_*` behavior is unchanged.

## What this is not yet

- automatic lock expiry or force-unlock
- a CLI `apply-lock-clear` / `rm` helper
- database advisory locks
- daemon-local mutation endpoints
- multi-host / orchestrated unlock coordination
- ground-item restart durability or SQL import/backfill
- treating `manual_clear_candidate=true` alone as permission to delete the lock

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing lock still returns `present: false` without candidate fields and without opening a DB;
- lock matching the lab candidate gate (absent PID, local hostname, matching build identity, age ≥ 3600s) returns `manual_clear_candidate: true` and `manual_clear_check: lab_stale_lock_policy_v1`;
- locks that fail any one gate (alive PID, foreign hostname, foreign build, age < 3600) return `manual_clear_candidate: false`;
- status still leaves the lock file in place and never prints DSN/SQL text;
- existing PID-liveness / hostname-locality / build-match / age / malformed / symlink coverage remains green.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyLockStatus' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal deferred; any future clear helper must require explicit operator confirmation beyond this advisory bit.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
4. Add systemd/unit or multi-host unlock samples only after the lab topology has been exercised on a real host.
