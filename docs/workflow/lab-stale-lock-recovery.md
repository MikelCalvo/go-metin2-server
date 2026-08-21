# Lab Stale-Lock Recovery Policy — 2026-08-21

## Objective

Freeze the first single-host lab recovery policy for leftover `metin2-migrate apply --lock-file` artifacts. This is operator judgment plus an advisory triage bit — not automatic unlock and not a distributed mutex.

Use this only on the [lab deployment topology](lab-deployment-topology.md) where `authd`, `gamed`, and `metin2-migrate` share one host and migration apply stays CLI-only.

## Inspect first

When `apply --lock-file <path>` fails because the lock already exists:

```bash
metin2-migrate apply-lock-status --lock-file <path> > apply-lock-status.json
```

Retain that JSON under the current `/var/metin2/migration-runs/YYYY-MM-DDTHHMMSSZ-<commit12>/` tree beside the other metadata-safe artifacts. The helper never deletes the lock, never opens the DB target, and never prints DSNs or executable SQL.

Present locks report:

| Field | Meaning |
| --- | --- |
| `holder_pid_alive` / `holder_pid_check=local_signal_0` | advisory local signal-0 existence probe against `lock.pid` |
| `holder_hostname_local` / `holder_hostname_check=local_os_hostname` | exact trimmed compare of `lock.hostname` to inspecting `os.Hostname()` |
| `holder_build_matches` / `holder_build_check=local_buildinfo_current` | exact trimmed compare of stamped build identity to `buildinfo.Current()` |
| `lock_age_seconds` / `lock_age_check=local_wall_clock` | non-negative whole-second floor of age since `lock.created_at` (future clamps to `0`) |
| `manual_clear_candidate` / `manual_clear_check=lab_stale_lock_policy_v1` | advisory lab gate defined below |

## Lab manual-clear candidate gate

`manual_clear_candidate` is `true` only when **all** of these hold on the inspecting host:

1. `holder_pid_alive == false`
2. `holder_hostname_local == true`
3. `holder_build_matches == true`
4. `lock_age_seconds >= 3600`

Anything else — alive PID, foreign hostname, foreign/unstamped build identity, or age under one hour — stays `manual_clear_candidate: false`. Treat every probe as triage evidence only: PID reuse, container namespaces, copied lock files, clock skew, and long-running intentional applies still require operator judgment.

A `true` candidate does **not** authorize `rm`, does **not** unlock the database, and does **not** replace retained preflight / audit / backup review.

## Manual recovery steps (lab only)

Only after `manual_clear_candidate` is `true` **and** the operator has reviewed retained artifacts:

1. Confirm no other operator still owns the migration window (notes, chat, host console).
2. Confirm DB and file-store backup evidence for the window still exists outside the live data trees.
3. Compare `lock.plan_sha256` / `lock.ledger_snapshot_sha256` / stamped build identity against retained `migration-plan-artifact.json`, `apply-preflight.json`, and `GET /local/build-info` / `metin2-migrate version`.
4. Aside-rename through the confirmation-gated CLI helper (preferred over hand-rolled `mv`):

```bash
metin2-migrate apply-lock-aside \
  --lock-file <path> \
  --i-confirm-lab-aside-rename \
  > apply-lock-aside.json
```

The helper recomputes the lab gate immediately before renaming, writes `<path>.stale-<UTC>` (for example `.stale-20260821T153045Z`), refuses destination collisions, never unlinks the lock, and never opens the DB target. Retain `apply-lock-aside.json` beside `apply-lock-status.json` in the migration-runs tree.

If the CLI binary is unavailable and an operator must fall back to a manual rename after the same review, keep the same destination naming:

```bash
ts=$(date -u +%Y%m%dT%H%M%SZ)
mv -- "<path>" "<path>.stale-${ts}"
```

5. Retain the renamed lock beside `apply-lock-status.json` / `apply-lock-aside.json` in the migration-runs tree.
6. Re-run `apply-lock-status` and confirm `present: false` before starting a fresh `apply --lock-file` with a new lock path or the original path now free.
7. Re-validate the reviewed plan/preflight boundary before opening the DB again (`plan-artifact-status` / `apply-preflight-status` as appropriate).

Do **not**:

- use `rm` as the first action;
- clear a lock while `holder_pid_alive` is `true`;
- clear a lock copied from another host (`holder_hostname_local=false`);
- clear a lock from a different binary stamp (`holder_build_matches=false`) without proving the original apply finished or rolled back;
- invent a daemon `/local/...` unlock endpoint;
- treat `manual_clear_candidate=true` alone as enough without `--i-confirm-lab-aside-rename` (or equivalent operator judgment for a manual fallback);
- treat this policy as multi-host or orchestrated unlock coordination.

## Relationship to other docs

- [migration apply runbook](migration-apply-runbook.md) — forward/rollback ordering and failure handling
- [lab deployment topology](lab-deployment-topology.md) — host layout and artifact retention trees
- [file-store backup/restore drill](file-store-backup-restore-drill.md) — JSON store backup evidence before mutation windows

## What this is not yet

- automatic stale-lock expiry or daemon/cron unlock
- `rm` / unlink / truncate helpers
- DB-engine advisory locks
- multi-host unlock coordination
- a claim that leftover locks prove a migration succeeded or failed
- treating `manual_clear_candidate=true` alone as permission to mutate without confirmation / operator judgment
