# CLI Migration Rollback Confirmation Guard — 2026-08-16

## Objective

Harden the CLI-only migration apply boundary so rollback/down plans cannot be executed solely by passing a lower `--target-version`.

The repo already had strict ledger snapshots, metadata-only dry-run plans, plan artifacts, optional local lock files, optional audit files, and a CLI-only `metin2-migrate apply` command. This slice keeps the mutating surface outside daemon ops endpoints while adding one explicit rollback acknowledgement before any database connection is opened.

## Contract frozen by this slice

`metin2-migrate apply` still accepts up targets exactly as before:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version latest \
  --plan-artifact migration-plan-artifact.json \
  --lock-file migration-apply.lock \
  --audit-file migration-apply-audit.json
```

Any plan containing one or more down/rollback steps is now rejected unless the operator also passes `--allow-rollback`:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version 0 \
  --plan-artifact rollback-plan-artifact.json \
  --lock-file migration-apply.lock \
  --audit-file migration-rollback-audit.json \
  --allow-rollback
```

The guard runs after the offline ledger snapshot has been decoded and the metadata-only plan has been computed, but before plan-artifact file reads, lock-file reservation, audit-file reservation, `sql.Open`, transaction begin, or migration SQL execution. That ordering means a mistyped rollback target fails closed before touching local coordination files or the database.

The `--allow-rollback` flag is intentionally separate from:

- the numeric `--target-version` value;
- `--plan-sha256`;
- `--plan-artifact`;
- `--lock-file`;
- `--audit-file`.

Plan confirmation proves the plan matches what was reviewed; it does not by itself authorize destructive direction. The rollback acknowledgement makes the dangerous direction explicit in the apply command line.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply` or `/local/db/migrations/rollback`;
- daemon startup auto-migration;
- bundled production database drivers or engine selection;
- backup/restore orchestration around rollback;
- stale-lock recovery, distributed/advisory DB locks, or multi-operator policy;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

## Why this order

The CLI already had enough machinery to perform down migrations. That is useful for tested recovery drills, but a rollback is operationally more dangerous than applying pending up migrations. Requiring `--allow-rollback` is a small production-safety improvement that does not require selecting a DB engine, inventing a remote admin surface, or broadening runtime persistence.

This guard complements the existing recommended runbook flow:

1. export or create a strict ledger snapshot;
2. generate and inspect a metadata-only plan artifact;
3. take and validate backups through the existing file-store preflight surfaces where applicable;
4. run `metin2-migrate apply` with plan confirmation, a local lock, and an audit file;
5. add `--allow-rollback` only for an intentional rollback/down plan.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- rollback target `0` succeeds only when `--allow-rollback` is present;
- the same rollback plan without `--allow-rollback` fails closed before opening the database;
- the guard fires before plan-artifact reads, lock-file reservation, or audit-file reservation;
- missing ledger snapshots for rollback remain usage errors before DB open.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(UsesOfflineLedgerSnapshotForRollbackTarget|RejectsRollbackTargetWithoutAllowRollbackBeforeOpeningDatabase|RejectsRollbackBeforePlanArtifactLockAuditTouch|RejectsMissingLedgerSnapshotForRollbackTarget)' -count=1`
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add a production migration runbook that sequences ledger export, plan artifact review, backup validation, `apply --plan-artifact --lock-file --audit-file`, and `--allow-rollback` only for explicit rollback drills.
2. Add driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add advisory-lock or stale-lock recovery policy after deployment topology is known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
