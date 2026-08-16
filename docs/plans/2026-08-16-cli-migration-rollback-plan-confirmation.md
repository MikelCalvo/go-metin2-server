# CLI Migration Rollback Plan Confirmation Guard — 2026-08-16

## Objective

Tighten the CLI-only migration rollback boundary so an operator cannot execute a down-migration plan with `--allow-rollback` alone.

The previous slice already required `--allow-rollback` for any plan containing down steps. This slice keeps that acknowledgement and adds a second explicit guard: rollback plans must also be tied to a previously inspected metadata-only plan through either `--plan-sha256` or `--plan-artifact`.

## Contract frozen by this slice

Up/forward apply behavior is unchanged. Operators may still apply pending up migrations with the normal reviewed-plan workflow:

```bash
metin2-migrate plan-artifact \
  --ledger-snapshot ledger.json \
  --target-version latest \
  > migration-plan-artifact.json

metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger.json \
  --target-version latest \
  --plan-artifact migration-plan-artifact.json \
  --lock-file migration-apply.lock \
  --audit-file migration-apply-audit.json
```

Any rollback/down plan now requires both:

1. direction acknowledgement: `--allow-rollback`;
2. plan confirmation: either `--plan-sha256 <hex>` or `--plan-artifact <path>`.

Recommended rollback flow:

```bash
metin2-migrate plan-artifact \
  --ledger-snapshot ledger.json \
  --target-version 0 \
  > rollback-plan-artifact.json

metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger.json \
  --target-version 0 \
  --plan-artifact rollback-plan-artifact.json \
  --lock-file migration-rollback.lock \
  --audit-file migration-rollback-audit.json \
  --allow-rollback
```

A rollback plan with `--allow-rollback` but without `--plan-sha256` or `--plan-artifact` fails closed before:

- reading a plan artifact path;
- reserving a lock file;
- reserving an audit file;
- opening the database;
- beginning a transaction;
- executing migration SQL.

The existing guard order remains intact: rollback without `--allow-rollback` still fails before artifact, lock, audit, or DB work even if `--plan-artifact` was provided.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply` or any daemon-local mutating migration endpoint;
- daemon startup auto-migration;
- bundled production database driver selection;
- backup orchestration around rollback;
- distributed/advisory DB locks or stale-lock recovery;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

## Why this order

Rollback is the riskiest migration direction. `--allow-rollback` proves the operator intentionally chose a destructive direction, but it does not prove the exact down plan was reviewed. Requiring a plan checksum or artifact for rollback gives the CLI a minimal two-factor operational confirmation without widening daemon privileges or selecting a production database engine.

This complements the current runbook posture:

1. export or create a strict ledger snapshot;
2. generate and inspect a metadata-only plan artifact;
3. validate relevant file-store backups/preflights;
4. run `metin2-migrate apply` with a plan artifact/checksum, lock file, and audit file;
5. add `--allow-rollback` only for intentional rollback drills.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- rollback target `0` succeeds when both `--allow-rollback` and matching `--plan-sha256` are present;
- rollback target `0` fails closed when `--allow-rollback` is present but no plan confirmation is supplied;
- the missing-plan-confirmation guard runs before lock/audit reservation or DB open;
- the previous missing-`--allow-rollback` guard still runs before artifact/lock/audit handling.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(UsesOfflineLedgerSnapshotForRollbackTarget|RejectsRollbackWithoutPlanConfirmationBeforeOpeningDatabase|RejectsRollbackTargetWithoutAllowRollbackBeforeOpeningDatabase|RejectsRollbackBeforePlanArtifactLockAuditTouch)' -count=1`
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add a single production migration runbook that sequences ledger snapshot export, plan artifact review, file-store backup validation, rollback confirmation, lock, apply, and audit retention.
2. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add advisory locking and stale-lock policy only after deployment topology is known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
