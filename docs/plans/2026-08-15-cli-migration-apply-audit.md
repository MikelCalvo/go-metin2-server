# CLI Migration Apply Audit Boundary — 2026-08-15

## Objective

Add a small audit artifact to the CLI-only migration apply path so an operator can keep a durable, metadata-only record of what an explicit migration run applied without exposing DSNs or executable SQL and without widening daemon ops endpoints.

The project already had a validated migration catalog, strict offline ledger snapshots, metadata-only dry-run planning, read-only daemon migration endpoints, and a CLI-only `metin2-migrate apply` command. This slice keeps the mutating surface CLI-only while adding an optional `--audit-file` guardrail for production runbooks.

## Contract frozen by this slice

`metin2-migrate apply` now accepts an optional audit output path:

```bash
metin2-migrate apply \
  --driver <database/sql-driver> \
  --dsn <dsn> \
  --ledger-snapshot <path|-> \
  --target-version <version|latest> \
  --audit-file migration-apply-audit.json
```

When `--audit-file` is omitted, `apply` keeps the existing stdout-only metadata result behavior.

When `--audit-file` is present:

- the ledger snapshot is still bounded to 64 KiB before decode/planning/database open;
- the snapshot must decode as strict `go-metin2-schema-migrations-ledger-v1` JSON;
- the target plan must contain at least one pending migration step;
- the audit path is reserved with exclusive file creation (`O_EXCL`) before the database is opened;
- existing audit files fail closed and are not overwritten;
- missing parent directories fail closed instead of being created implicitly;
- if migration apply fails after reservation, the empty reserved audit file is removed;
- on success, the audit file is written with mode `0600`, fsynced, and closed;
- stdout remains the normal metadata-only `ApplyResult` JSON.

The audit JSON uses format marker `go-metin2-migration-apply-audit-v1` and contains:

- `applied_at` UTC timestamp;
- configured driver name;
- `dsn_configured` boolean, never the DSN itself;
- resolved numeric `target_version`;
- whether the operator supplied `latest` as the target;
- SHA-256 of the exact offline ledger-snapshot bytes supplied to this run;
- the same metadata-only `ApplyResult` written to stdout.

The audit file never includes executable SQL text, migration SQL bodies, runtime store rows, or the supplied DSN.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or any daemon mutation endpoint;
- daemon startup auto-migration;
- a bundled production database driver or default engine;
- backup/restore orchestration around migration apply;
- advisory locks or multi-operator coordination;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

`--audit-file` is an operator artifact helper, not a lock, not a backup, and not a substitute for a tested deployment runbook.

## Why this order

The current apply command is intentionally explicit and CLI-only, but production operations still need evidence of what was run. A metadata audit file is smaller and safer than adding daemon mutation endpoints or selecting a database engine prematurely. Requiring pending steps prevents empty/no-op audit files from looking like successful migration events.

Reserving the audit path before opening the DB avoids a dangerous failure mode where migrations commit successfully but the chosen audit path was already occupied. Removing the reserved file on apply failure avoids leaving an empty artifact that looks like a successful run.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- successful audited apply writes strict metadata-only audit JSON;
- the audit result matches stdout metadata;
- the supplied DSN and executable SQL are absent from the audit file;
- no-op audited apply is rejected and writes no audit file;
- existing audit files fail closed before opening the database;
- failed apply removes the reserved audit file;
- missing audit parent directories fail closed before opening the database.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(WritesAuditFile|RejectsAuditFile|RemovesReservedAuditFile|RejectsOversizedLedgerSnapshot)' -count=1`;
- `go test ./internal/migratecli -count=1`;
- broader repo validation is recorded in the commit/run summary.

## Follow-up options

1. Add a backup/restore preflight runbook that requires `ledger-snapshot -> plan -> backup/validate -> apply --audit-file` ordering before recommending real deployments.
2. Add build-tagged driver-backed integration tests once a concrete DB engine/driver is selected.
3. Add advisory-lock or single-writer coordination after deployment topology and DB engine are known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
