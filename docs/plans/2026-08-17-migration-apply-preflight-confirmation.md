# Migration Apply Preflight Confirmation — 2026-08-17

## Objective

Let the mutating `metin2-migrate apply` command consume the retained `go-metin2-migration-apply-preflight-v1` artifact produced by `metin2-migrate apply-preflight`, so a migration window can require the exact no-side-effect preflight JSON reviewed immediately before backup validation and mutation.

The existing runbook already created a strict ledger snapshot, a metadata-only plan artifact, an apply preflight artifact, optional status snapshots, a local lock, and a metadata-only apply audit. Before this slice, the final `apply` command could re-check a plan checksum or plan artifact, but could not directly require that the reviewed preflight artifact matched the same ledger snapshot, target, and plan about to be applied.

## Contract frozen by this slice

`metin2-migrate apply` now accepts an optional retained preflight artifact:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  --apply-preflight apply-preflight.json \
  --lock-file migration-apply.lock \
  --audit-file migration-apply-audit.json
```

The `--apply-preflight <path>` option:

- is mutually exclusive with `--plan-sha256` and `--plan-artifact` so each apply uses one exact reviewed-plan confirmation source;
- reads the same strict `go-metin2-migration-apply-preflight-v1` JSON accepted by `apply-preflight-status`;
- rejects symlinks, non-regular files, oversized files over 128 KiB, invalid UTF-8, empty files, malformed JSON, unknown fields, trailing JSON, unsupported formats, checksum drift, invalid plan shape, and target/plan endpoint drift;
- recomputes the requested plan from the supplied offline ledger snapshot and target before any DB open;
- requires the preflight's `ledger_snapshot_sha256` to match the exact ledger snapshot bytes passed to `apply`;
- requires the preflight's resolved `target_version`, `target_latest`, `plan_sha256`, and embedded metadata-only plan to match the requested apply boundary;
- records the preflight plan checksum as `confirmed_plan_sha256` in the optional audit file;
- never trusts the preflight artifact instead of the live transaction-local `schema_migrations` verification performed by `db/migrations.ApplyToVersion`.

Rollback/down plans now accept the retained preflight artifact as the reviewed-plan confirmation source, but still require the explicit direction acknowledgement:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger-snapshot.json \
  --target-version <rollback-version> \
  --apply-preflight rollback-apply-preflight.json \
  --allow-rollback \
  --lock-file migration-rollback.lock \
  --audit-file migration-rollback-audit.json
```

A rollback plan without `--allow-rollback` still fails before reading plan/preflight artifacts, reserving lock/audit files, opening the database, or executing SQL. A rollback plan with `--allow-rollback` still requires one exact reviewed-plan confirmation source: `--plan-sha256`, `--plan-artifact`, or `--apply-preflight`.

## What this is not yet

This is not database backup validation, stale-lock recovery, DB advisory locking, daemon startup auto-migration, or a daemon-local mutation endpoint. It deliberately does not add:

- `/local/db/migrations/apply` or `/local/db/migrations/rollback`;
- bundled production DB driver selection;
- automatic use of a retained preflight without an explicit `apply` flag;
- runtime DB-backed account, character, item, quest, content, login-ticket, or world repositories.

The preflight artifact is an apply-input confirmation guard. Operators must still keep deployment-specific database backup evidence, file-store validation/backup evidence, lock/audit artifacts, and post-apply status output together for a complete migration window record.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `apply --apply-preflight <path>` accepts a matching preflight artifact and proceeds to the database mutation;
- audited apply records the preflight plan checksum in `confirmed_plan_sha256`;
- mismatched or malformed preflight artifacts fail closed before opening the database;
- `apply` rejects `--apply-preflight` combined with `--plan-sha256` or `--plan-artifact` as a usage error;
- a rollback/down apply can use the retained preflight artifact as its reviewed-plan confirmation only when `--allow-rollback` is present.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(UsesApplyPreflightArtifactBeforeMutation|RejectsMismatchedApplyPreflightBeforeOpeningDatabase|WritesApplyPreflightPlanSHA256IntoAuditFile|RejectsInvalidApplyPreflightBeforeOpeningDatabase|UsesApplyPreflightForRollbackTarget|RejectsRollbackWithoutPlanConfirmationBeforeOpeningDatabase|RejectsPlanArtifactAndPlanSHA256TogetherAsUsageError|RejectsApplyPreflightWithPlanConfirmationTogetherAsUsageError)' -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add deployment-specific artifact naming and retention rules once production topology is known.
2. Add DB-engine-specific advisory lock coverage once a production driver is selected.
3. Add build-tagged integration coverage for the selected DB engine and backup/restore workflow.
4. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
