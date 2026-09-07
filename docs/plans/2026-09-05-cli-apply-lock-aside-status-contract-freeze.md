# CLI apply-lock-aside-status contract freeze — 2026-09-05

## Objective

Freeze a read-only `metin2-migrate apply-lock-aside-status` inspector for
retained `apply-lock-aside.json` artifacts so operators can re-check a
confirmation-gated lab stale-lock aside-rename during incident review
**without** renaming another lock, opening a database, or trusting a
hand-edited aside file.

This freeze does **not** invent automatic stale-lock expiry, `rm` / unlink,
a stock production driver, automatic / scheduled script execution, loopback
ops mutation, or any claim that a present valid aside JSON proves a live
migration finished or that the original lock path is currently free.

## Why docs-first

Track E tip chain through `catalog-status` is Done
([CLI catalog-status contract freeze](2026-09-05-cli-catalog-status-contract-freeze.md)).

Every other retained migrate artifact already has a matching `*-status`
inspector (`catalog-status`, `plan-artifact-status`, `ledger-snapshot-status`,
`apply-preflight-status`, `apply-lock-status`, `apply-audit-status`,
`import-export-status`, `synthesize-wipe-export-status`,
`export-tree-status`, `export-tree-status-status`).
`apply-lock-aside` already emits metadata-only
`go-metin2-migration-apply-lock-aside-v1` JSON and the lab recovery runbook
retains `apply-lock-aside.json` beside `apply-lock-status.json`, but there
is no small command to re-validate that file by itself after the original
binary is archived, copied, or hand-edited.

`migration-run-retention` already **echoes** (does not auto-run)
`apply-lock-aside --i-confirm-lab-aside-rename` into leftover-lock triage.
GREEN must keep that confirmation gate: do **not** auto-run aside-rename
under `set -eu`.

Opening RED without freezing:

- exact command / flag names,
- outer status envelope + checksum field,
- size cap and fail-closed file policy,
- inner aside-path / renamed-at / lock / candidate consistency,
- whether live PID / hostname / build / age probes may be re-run,
- optional local-path existence gates,
- which printed leftover-lock lines adopt the new inspector,

would invent operator-facing incident-review semantics mid-implementation.
Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Command and flags (exact names frozen)

```bash
metin2-migrate apply-lock-aside-status \
  --aside <path> \
  [--require-aside-path-exists] \
  [--require-lock-file-absent]
```

Rules:

1. Requires `--aside`; extra positional arguments are usage errors
   (exit `2`).
2. The path is a retained `go-metin2-migration-apply-lock-aside-v1` JSON
   file (`apply-lock-aside.json`), **not** the live lock file and **not**
   stdin. Relative paths are allowed (same file-inspector policy as
   `apply-lock-status` / `apply-audit-status`). stdin (`-`) is **not**
   accepted: a path whose name is `-` is a missing/regular file named `-`.
3. `--require-aside-path-exists` and `--require-lock-file-absent` are
   independently opt-in (boolean; default false).
4. Usage text lists the command beside `apply-lock-aside` /
   `apply-lock-status` and lists both require flags beside `--aside`.
5. Unknown flags / unexpected args still exit `2`.
6. Performs no database open, SQL execution, lock reservation, lock
   rename, lock deletion, audit reservation, daemon mutation, or
   `rm` / unlink.
7. Never emits DSNs, executable SQL, runtime store rows, or live ledger
   payloads.
8. Never re-runs live `holder_pid_alive` / hostname / build / age probes.
   Retained triage fields are historical evidence from the aside instant.

### B. File presence and fail-closed I/O

1. Returns success with outer `present: false` (and no inner `aside`) when
   the path is absent.
2. Rejects symlink or non-regular paths, oversized files over **16 KiB**
   (same cap as `apply-lock-status`), invalid UTF-8, empty files, malformed
   JSON, unknown fields (`DisallowUnknownFields`), trailing JSON, or an
   unsupported `format` marker with exit `1`, a short stderr reason, and
   **no** stdout status JSON.
3. Input `format` must be `go-metin2-migration-apply-lock-aside-v1`. A
   `go-metin2-migration-apply-lock-v1` lock file, a
   `go-metin2-migration-apply-lock-status-v1` status file, or a
   `go-metin2-migration-apply-lock-aside-status-v1` file (this command's
   own output) is the wrong format and must fail closed.
4. `aside_sha256` is computed over the **exact retained file bytes** so
   operators can correlate the inspected file with lab notes / retention
   trees.

### C. Inner `present: false` retained snapshots

There is no valid inner `present: false` aside shape. Missing-path
handling is only the outer envelope in section E.

Any selected require-gate against an absent path fails closed with exit
`1`, a short stderr reason that names the failed gate / absent aside, and
**no** stdout JSON.

### D. Inner aside consistency (no DB open, no live holder re-probe)

When the file is present, decode into the existing
`migrationApplyLockAside` shape
(`go-metin2-migration-apply-lock-aside-v1`) and fail closed unless **all**
of the following hold:

1. `lock_file` is a non-empty trimmed path.
2. `aside_path` is a non-empty trimmed path.
3. `renamed_at` parses as RFC3339Nano.
4. `aside_path` equals
   `lock_file + ".stale-" + renamed_at.UTC().Format("20060102T150405Z")`
   (same destination rule as `lockAsidePath`).
5. Inner `lock` is present and passes the same
   `normalizeMigrationApplyLock` rules already owned by
   `apply-lock-status` (format, created_at, positive pid, hostname,
   complete build identity, driver, `dsn_configured=true`,
   non-negative target, latest-target positive version, lowercase hex
   SHA-256 checksums).
6. Historical triage fields are all present:
   - `holder_pid_alive` / `holder_pid_check=local_signal_0`
   - `holder_hostname_local` / `holder_hostname_check=local_os_hostname`
   - `holder_build_matches` / `holder_build_check=local_buildinfo_current`
   - `lock_age_seconds` / `lock_age_check=local_wall_clock`
     (`lock_age_seconds >= 0`)
   - `manual_clear_candidate` / `manual_clear_check=lab_stale_lock_policy_v1`
7. `manual_clear_candidate` is `true`. Aside-rename only succeeds when the
   lab gate held; a retained false-candidate aside is not a valid artifact.
8. The decoded object contains **no** DSN or executable SQL text fields.

This is metadata-only evidence that the **retained JSON is internally
consistent**. It does not prove the live process table, hostname, or
build identity still match, and it does not prove a database is migrated.

After consistency succeeds, set local filesystem existence bits **without**
following the aside-rename mutation path:

- `aside_path_exists` is `true` only when `aside_path` is a present
  non-symlink regular file.
- `lock_file_exists` is `true` when `lock_file` exists (`Lstat` does not
  return `ErrNotExist`).

Then apply require-gates when selected, each fail-closed with exit `1` and
**no** stdout JSON:

- `--require-aside-path-exists` when `aside_path_exists` is false.
- `--require-lock-file-absent` when `lock_file_exists` is true.

Ungated missing existence bits are still success: operators can inspect an
archived aside JSON after the `.stale-*` lock copy has been moved off the
lab host.

### E. Successful outer envelope

```json
{
  "format": "go-metin2-migration-apply-lock-aside-status-v1",
  "present": true,
  "aside_sha256": "...",
  "aside_path_exists": true,
  "lock_file_exists": false,
  "aside": {
    "format": "go-metin2-migration-apply-lock-aside-v1",
    "lock_file": "/var/metin2/migration-runs/.../migration-apply.lock",
    "aside_path": "/var/metin2/migration-runs/.../migration-apply.lock.stale-20260821T153045Z",
    "renamed_at": "2026-08-21T15:30:45Z",
    "lock": {
      "format": "go-metin2-migration-apply-lock-v1"
    },
    "holder_pid_alive": false,
    "holder_pid_check": "local_signal_0",
    "holder_hostname_local": true,
    "holder_hostname_check": "local_os_hostname",
    "holder_build_matches": true,
    "holder_build_check": "local_buildinfo_current",
    "lock_age_seconds": 3600,
    "lock_age_check": "local_wall_clock",
    "manual_clear_candidate": true,
    "manual_clear_check": "lab_stale_lock_policy_v1"
  }
}
```

When the path is absent:

```json
{
  "format": "go-metin2-migration-apply-lock-aside-status-v1",
  "present": false
}
```

No extra JSON fields on the outer envelope. Inner `aside` is the decoded
retained `go-metin2-migration-apply-lock-aside-v1` object (same field names
as live `metin2-migrate apply-lock-aside` stdout). Omit inner existence /
checksum fields on the missing-path envelope.

### F. Printer wiring (same GREEN as the inspector)

GREEN must **not** auto-run `apply-lock-aside` or
`apply-lock-aside-status` under `set -eu`. Successful apply removes the
lock; leftover-lock triage already only runs `apply-lock-status` when
`"$RUN/$LOCK_FILE"` still exists and echoes aside-rename as an
operator-run hint.

Add a matching **echoed** `apply-lock-aside-status` hint immediately after
the existing aside-rename echo in `migration-run-retention` (forward and
`--allow-rollback`):

```sh
echo "  metin2-migrate apply-lock-aside --lock-file \"$RUN/$LOCK_FILE\" --i-confirm-lab-aside-rename > \"$RUN/apply-lock-aside.json\""
echo "  metin2-migrate apply-lock-aside-status --aside \"$RUN/apply-lock-aside.json\" > \"$RUN/apply-lock-aside-status.json\""
```

Do **not** add `--require-*` flags on the printed hint: operators may
inspect an archived aside JSON after the `.stale-*` copy has moved.

Do **not** auto-run ungated missing-file `apply-lock-aside-status` on the
happy path. Hermetic SQLite proofs keep expecting no
`apply-lock-aside-status.json` after a successful apply (lock is gone,
aside never ran).

Contrib `lab-retention-gc` forwarding of `migration-run-retention` stays
unchanged in this freeze (no new env vars required here).

### G. Explicit non-goals

- re-running live PID / hostname / build / age probes
- renaming, unlinking, or truncating lock / aside files
- exposing executable SQL or DSNs
- upsert / merge / tip-`0002` single-pass cascade-delete
- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- automatic / scheduled execution of printed aside / apply / import scripts
- opening a database from `apply-lock-aside-status`
- claiming a present aside JSON proves a live database is migrated or that
  the original lock path is currently free
- changing leftover-lock triage to auto-run `apply-lock-aside`
- changing default (ungated) missing-file outer `present: false` exit `0`
- loopback ops mutation endpoint / remote admin / secrets in git / metrics
- broad README churn

## Likely files for GREEN (not this freeze)

- `internal/migratecli/apply_lock_aside_status.go` (new)
- `internal/migratecli/apply_lock_aside_status_test.go` (new)
- `internal/migratecli/migratecli.go` (command switch + usage; reuse
  `normalizeMigrationApplyLock` / `lockAsidePath`)
- `internal/migratecli/migration_run_retention.go` (echoed status hint)
- `internal/migratecli/migration_run_retention_test.go`
- `docs/development.md`
- `docs/debugging-and-profiling.md`
- `docs/workflow/migration-apply-runbook.md`
- `docs/workflow/lab-stale-lock-recovery.md`
- `docs/workflow/lab-deployment-topology.md` (list
  `apply-lock-aside-status.json` only once GREEN produces it)
- Track E pointers in `docs/plans/2026-08-08-playable-vertical-roadmap.md` /
  `docs/plans/2026-08-09-db-migration-contract.md`
- this plan (flip freeze → Done on GREEN)

## TDD plan for GREEN (after this freeze)

Focused coverage in `internal/migratecli`:

- missing aside path → outer `present: false`, no DB open
- valid retained aside JSON with matching `.stale-<UTC>` destination →
  outer `present: true` + checksum + existence bits + inner aside, no DB
  open, no live holder re-probe
- `--require-aside-path-exists` / `--require-lock-file-absent` succeed when
  the renamed lock is present as a regular file and the original path is
  gone
- `--require-aside-path-exists` with missing/symlink aside path → exit `1`,
  empty stdout
- `--require-lock-file-absent` when the original lock path still exists →
  exit `1`, empty stdout
- require-gates against an absent aside JSON → exit `1`, empty stdout
- `aside_path` / `renamed_at` mismatch, false `manual_clear_candidate`,
  missing triage fields, invalid lock object, unknown-field / wrong format
  (including this command's own outer envelope, a raw lock file, or
  `apply-lock-status` JSON) / symlink / oversized → exit `1`, empty stdout
- usage / unknown-command mention `apply-lock-aside-status`
- `migration-run-retention` forward + rollback printers echo the matching
  `apply-lock-aside-status --aside` hint and still do **not** auto-run
  `apply-lock-aside` or `apply-lock-aside-status`

Validation for GREEN:

```bash
go test ./internal/migratecli -run 'ApplyLockAsideStatus|MigrationRunRetentionPrints|RejectsUnknownCommandMentionsApplyLockAsideStatus' -count=1
gofmt -l internal/migratecli/*.go
git diff --check
```

Hermetic `MigrationRunRetentionSQLite` stays a no-behavior-change proof for
this slice unless leftover-lock aside coverage is added later.

## Status

GREEN on `lane/persistence`.

- Read-only `metin2-migrate apply-lock-aside-status --aside <path>
  [--require-aside-path-exists] [--require-lock-file-absent]` re-validates
  retained `apply-lock-aside.json` without opening a database, renaming a
  lock, or re-running live PID / hostname / build / age probes.
- Outer envelope is `go-metin2-migration-apply-lock-aside-status-v1`; missing
  path is ungated `present: false`; present files re-check inner
  `aside_path` / `renamed_at` destination, lock object, historical triage
  fields, and `manual_clear_candidate=true`, then report local
  `aside_path_exists` / `lock_file_exists` bits.
- `migration-run-retention` (forward + rollback) leftover-lock triage echoes
  a matching `apply-lock-aside-status --aside` hint beside the existing
  confirmation-gated aside-rename echo and still does **not** auto-run
  aside-rename or aside-status under `set -eu`.
- Upsert / auto-run / stock production driver / cascade-delete remain deferred.
- Follow-up owned separately and now GREEN: read-only `backup-tree-status` for retained backup-restore trees — see [CLI backup-tree-status contract freeze](2026-09-06-cli-backup-tree-status-contract-freeze.md).
- Follow-up owned separately and now GREEN: read-only `backup-tree-status-status` for retained `backup-tree-status.json` — see [CLI backup-tree-status-status contract freeze](2026-09-06-cli-backup-tree-status-status-contract-freeze.md).
- Follow-up owned separately and now GREEN: opt-in `--require-no-crash-temps` on `backup-tree-status` / `backup-tree-status-status` — see [CLI backup-tree-status no-crash-temps require-gate contract freeze](2026-09-06-cli-backup-tree-status-no-crash-temps-require-gate-contract-freeze.md).
- ~~Follow-up owned separately: freeze-only read-only `persistence-status-status` for retained `persistence-status-*.json`.~~ Done — see [CLI persistence-status-status contract freeze](2026-09-07-cli-persistence-status-status-contract-freeze.md). Matching `migration-run-retention` printer + hermetic curl-stub expansion is now GREEN — see [CLI migration-run-retention persistence-status-status](2026-09-07-cli-migration-run-retention-persistence-status-status-contract-freeze.md).
- Follow-up owned separately: freeze-only read-only `status-status` for retained `post-apply-status.json` / `post-rollback-status.json` — see [CLI status-status contract freeze](2026-09-07-cli-status-status-contract-freeze.md).

## Exit criteria for this freeze

- this plan exists and names exact command / flags / envelope / consistency
  rules / existence bits / printer wiring
- Track E / migration-contract point at this freeze as the next GREEN target
- no Go production code changes in the freeze commit
- tree stays green (`git status` clean after docs commit)

## Anti-goals / ordering constraints

- Do not open RED until this freeze is committed.
- Do not open a database, rename a lock, or re-probe live holders from the
  status command.
- Do not auto-run `apply-lock-aside` from printed retention scripts.
- Do not register a production driver or auto-run printed scripts.
- Do not invent cascade delete inside roster replace.
- Do not push `origin/main`; push only `origin/lane/persistence`.
- Do not list `apply-lock-aside-status.json` in lab topology or add a
  working CLI example until GREEN actually produces the command.
  GREEN now owns that listing and the leftover-lock inspect hint.
