# CLI Migration Apply Local Lock Boundary — 2026-08-15

## Objective

Add a small local single-writer guard to the CLI-only migration apply path so production runbooks can fail closed when another on-box migration run has already reserved the operator-chosen lock file.

The project already had strict ledger snapshots, metadata-only plan artifacts, optional apply audit files, and a CLI-only mutating `metin2-migrate apply` command. This slice keeps daemon ops endpoints read-only while adding an optional `--lock-file` guard before the CLI opens the database target.

## Contract frozen by this slice

`metin2-migrate apply` now accepts:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  --plan-artifact migration-plan-artifact.json \
  --lock-file migration-apply.lock \
  --audit-file migration-apply-audit.json
```

When `--lock-file` is omitted, `apply` keeps the previous CLI-only behavior.

When `--lock-file` is present:

- the ledger snapshot and optional plan artifact are validated first;
- the lock path is reserved with exclusive file creation (`O_EXCL`) before the database target is opened;
- an existing lock file fails closed, writes no stdout, preserves the existing file contents, and does not open the DB target;
- a successfully reserved lock is written with minimal local process metadata, synced, and held for the duration of the apply attempt;
- the reserved lock file is removed after both successful and failed apply attempts;
- apply output remains metadata-only and still redacts the supplied DSN from failures.

This lock is intentionally local filesystem coordination only. It is not a distributed lock, not a database advisory lock, not a stale-lock reaper, and not a replacement for a deployment-level single-writer policy.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or any daemon mutation endpoint;
- daemon startup auto-migration;
- a bundled production database driver or default engine;
- stale lock expiry or force-unlock tooling;
- distributed/advisory database locking;
- backup/restore orchestration around migration apply;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

## Why this order

The apply path is still an explicit operator action, but the runbook was missing a small guard against two local shells/scripts starting a migration at the same time. An exclusive local lock file is not a complete production coordination design, but it is a useful fail-closed boundary that does not require choosing a database engine or exposing daemon mutation endpoints.

Keeping the guard optional also preserves tests and workflows that use ephemeral in-memory drivers while giving future deployment docs a concrete place to reserve a host-local migration slot before opening the DB.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- an existing lock file rejects apply before opening the DB target and leaves the file untouched;
- a successful apply removes its reserved lock file;
- a failed apply removes its reserved lock file and still redacts the supplied DSN;
- locked apply output remains metadata-only and does not expose SQL, DSNs, or lock paths.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(RejectsExistingLockFile|RemovesLockFile|RemovesReservedLockFile)' -count=1`;
- broader repo validation is recorded in the commit/run summary.

## Follow-up options

1. Add a production migration runbook that sequences ledger export, plan artifact review, backup validation, `apply --plan-artifact --lock-file --audit-file`, and audit retention.
2. Add stale-lock inspection/recovery only after the deployment topology and operator policy are known.
3. Add database advisory locking or single-writer coordination after a concrete DB engine is selected.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
