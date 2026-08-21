# CLI Apply-Lock Aside Rename — 2026-08-21

## Objective

Encode the lab stale-lock recovery aside-rename as a fail-closed `metin2-migrate` helper so operators no longer hand-roll `mv` after `apply-lock-status` reports `manual_clear_candidate=true`.

`apply-lock-status` already computes the advisory lab gate. The runbook already requires aside-rename (not `rm`) after operator judgment. This slice adds an explicit confirmation-gated rename that rechecks that same gate immediately before mutation.

## Contract frozen by this slice

```bash
metin2-migrate apply-lock-aside \
  --lock-file <path> \
  --i-confirm-lab-aside-rename
```

Rules:

1. `--lock-file` is required. The path must be a present non-symlink regular `go-metin2-migration-apply-lock-v1` file under the existing 16 KiB decode cap.
2. `--i-confirm-lab-aside-rename` is required. Missing confirmation is a usage/fail-closed error and leaves the lock untouched.
3. Before renaming, the helper recomputes the same lab gate used by `apply-lock-status`:
   - `holder_pid_alive == false`
   - `holder_hostname_local == true`
   - `holder_build_matches == true`
   - `lock_age_seconds >= 3600`
4. If any gate fails, the command exits non-zero, leaves the lock in place, and does not create an aside path.
5. On success it renames to `<lock-file>.stale-<UTC compact timestamp>` where the timestamp is `YYYYMMDDTHHMMSSZ` from the inspecting host's UTC wall clock (same clock source as age triage).
6. Destination collision fails closed: if the aside path already exists, do not overwrite and leave the original lock in place.
7. The helper never opens a database, never executes SQL, never reserves audit/apply files, never prints DSNs, and never unlinks/`rm`s the lock.
8. Successful stdout is metadata-only `go-metin2-migration-apply-lock-aside-v1` JSON including:
   - `format`
   - `lock_file`
   - `aside_path`
   - `renamed_at` (RFC3339Nano UTC)
   - the same present-lock triage fields already owned by `apply-lock-status` (`lock`, holder probes, age, `manual_clear_candidate`, `manual_clear_check`)
9. `manual_clear_candidate` remains advisory evidence. The confirmation flag is intentionally separate and required even when the candidate bit is already true.

## What this is not yet

- automatic stale-lock expiry or daemon/cron unlock
- `rm` / unlink / truncate helpers
- DB-engine advisory locks or multi-host unlock coordination
- a `/local/...` unlock endpoint
- treating `manual_clear_candidate=true` alone as permission to mutate
- ground-item restart durability or SQL import/backfill

## TDD and validation

Focused coverage in `internal/migratecli`:

- missing confirmation leaves the lock untouched and exits non-zero
- lock matching the lab gate with confirmation is renamed to `.stale-<UTC>` and emits aside JSON
- alive PID and age < 3600 refuse rename and leave the lock in place
- destination collision fails closed
- missing lock fails closed without DB open
- helper never opens a database target
- existing `apply-lock-status` PID/hostname/build/age/malformed/symlink coverage remains green (shared inspect helper)

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApplyLockAside|TestRunApplyLockStatus' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep automatic stale-lock removal deferred.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Keep ground-item restart durability deferred until operators decide that quarantined `0010` exports should drive recovery.
4. Optional migration-runs retention printer for aside locks remains deferred; naming stays documented in the lab topology/runbook.
