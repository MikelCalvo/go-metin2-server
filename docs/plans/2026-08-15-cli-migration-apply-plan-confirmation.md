# CLI Migration Apply Plan Confirmation — 2026-08-15

## Objective

Add an optional plan-checksum confirmation guard to the CLI-only migration apply path so an operator can prove the mutating run still matches a previously inspected metadata-only dry-run plan before any database connection is opened.

The project already had strict ledger snapshots, `metin2-migrate plan`, CLI-only `metin2-migrate apply`, and optional apply audit files. This slice keeps migration mutation outside daemon ops endpoints while adding one more production-ops safety boundary around explicit apply runs.

## Contract frozen by this slice

`metin2-migrate apply` now accepts an optional plan checksum:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  --plan-sha256 <64-hex-dry-run-plan-sha256>
```

When `--plan-sha256` is omitted, `apply` keeps the existing explicit CLI-only mutation behavior.

When `--plan-sha256` is present:

- the ledger snapshot is still bounded to 64 KiB before decode/planning/database open;
- the snapshot must decode as strict `go-metin2-schema-migrations-ledger-v1` JSON;
- `--plan-sha256` must be exactly 64 hexadecimal characters or the command exits with usage error `2`;
- the CLI recomputes the metadata-only dry-run `Plan` for the supplied ledger snapshot and resolved target version;
- the checksum is computed over the same indented JSON shape emitted by `metin2-migrate plan`;
- a mismatch fails closed with exit `1` before opening the configured database;
- a match allows the existing apply path to open the database and execute the validated target plan;
- when combined with `--audit-file`, the audit JSON carries `confirmed_plan_sha256` so the post-run artifact links the applied result back to the inspected plan without including executable SQL or DSNs.

This guard is intended for runbooks that perform:

```bash
metin2-migrate ledger-snapshot --driver <driver> --dsn <dsn> > ledger.json
metin2-migrate plan --ledger-snapshot ledger.json --target-version latest > plan.json
sha256sum plan.json
metin2-migrate apply \
  --driver <driver> \
  --dsn <dsn> \
  --ledger-snapshot ledger.json \
  --target-version latest \
  --plan-sha256 <sha256-from-plan-json> \
  --audit-file migration-apply-audit.json
```

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or any daemon mutation endpoint;
- daemon startup auto-migration;
- a bundled production database driver or default engine;
- backup/restore orchestration around migration apply;
- advisory locks or multi-operator coordination;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

`--plan-sha256` is a confirmation guard, not a lock, not a backup, not a concurrency control mechanism, and not a substitute for taking and validating backups before mutating a real database.

## Why this order

The migration CLI can already mutate a target from an explicit offline ledger snapshot. The remaining production-safety risk is an operator accidentally applying a different target or catalog/ledger boundary than the plan they inspected. A checksum over the metadata-only plan is a small, engine-agnostic guard that catches that class of mistake before opening the database, without choosing a production DB engine or widening daemon ops surfaces.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `apply --plan-sha256 <matching>` accepts the same plan JSON checksum emitted by `metin2-migrate plan` and proceeds to mutation;
- `apply --plan-sha256 <matching> --audit-file <path>` records the confirmed plan checksum in the metadata-only audit JSON;
- mismatched plan checksums fail closed before opening the database;
- malformed plan checksums are usage errors and also fail before opening the database.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(AcceptsConfirmedPlanSHA256BeforeMutation|RejectsMismatchedPlanSHA256BeforeOpeningDatabase|RejectsMalformedPlanSHA256AsUsageError)' -count=1`;
- broader repo validation is recorded in the commit/run summary.

## Follow-up options

1. Add a backup/restore preflight runbook that requires `ledger-snapshot -> plan -> checksum -> backup/validate -> apply --plan-sha256 --audit-file` ordering before recommending real deployments.
2. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add advisory-lock or single-writer coordination after deployment topology and DB engine are known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
