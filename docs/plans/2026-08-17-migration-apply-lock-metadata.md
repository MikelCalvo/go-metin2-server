# Migration Apply Lock Metadata — 2026-08-17

## Objective

Tighten the CLI-only migration apply lock artifact so an operator inspecting an existing `--lock-file` can correlate it with the exact reviewed migration run boundary without exposing DSNs, executable SQL, or runtime store rows.

The prior lock slice reserved an exclusive local file and wrote only `pid=<pid>`. That was sufficient for single-writer coordination, but weak for safe handoff: when a lock existed, the file did not identify the target version, ledger snapshot, or dry-run plan the blocked/active run had reserved.

## Contract frozen by this slice

`metin2-migrate apply --lock-file <path>` still creates the lock only after:

1. strict offline ledger-snapshot decoding;
2. target-version resolution;
3. dry-run plan recomputation;
4. optional `--plan-sha256` / `--plan-artifact` validation;
5. rollback acknowledgement and reviewed-plan confirmation for down plans.

Before opening the database target, the reserved lock file is now strict metadata-only JSON with format marker `go-metin2-migration-apply-lock-v1`:

- `created_at` — UTC timestamp for the local reservation;
- `pid` — local process id of the reserving CLI process;
- `driver` — configured `database/sql` driver name;
- `dsn_configured` — boolean only, never the DSN value;
- `target_version` and `target_latest` — resolved target metadata;
- `ledger_snapshot_sha256` — checksum over the exact offline ledger snapshot bytes supplied to `apply`;
- `plan_sha256` — checksum over the exact dry-run plan recomputed for the target;
- optional `confirmed_plan_sha256` when the run used `--plan-sha256` or `--plan-artifact`.

The lock remains a local coordination guard:

- it is created with exclusive file creation (`O_EXCL`) and mode `0600`;
- an existing file still fails closed before database open and is not modified;
- a reserved lock is removed on successful or failed apply;
- the CLI still writes the normal metadata-only `ApplyResult` JSON to stdout on success.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or any daemon mutation endpoint;
- daemon startup auto-migration;
- stale-lock expiry, force-unlock, or process liveness policy;
- database advisory locks or distributed coordination;
- backup/restore orchestration inside the CLI;
- a bundled production database driver;
- DB-backed account, character, item, quest, content, login-ticket, or world repositories.

A stale lock is still an operator policy question. The metadata makes manual triage safer, but it does not authorize automatic deletion.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves that a locked apply writes metadata-only JSON before opening the database, includes the driver/target/plan/ledger correlation fields, excludes the DSN and executable SQL, and still removes the lock after success.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunApply(WritesMetadataOnlyLockFileBeforeOpeningDatabase|RejectsExistingLockFile|RemovesLockFile|RemovesReservedLockFile)' -count=1`;
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add a read-only stale-lock inspection helper only after deployment topology defines when a PID/host/checksum tuple is safe to treat as abandoned.
2. Add DB-engine-specific advisory lock coverage once the project selects a production driver.
3. Add import/quarantine tooling for schema-shaped exports before extracting DB-backed repositories.
4. Keep daemon-local migration endpoints read-only unless a future production-admin threat model intentionally changes that boundary.
