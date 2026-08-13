# Programmatic Migration Rollback Primitive — 2026-08-13

## Objective

Extend the package-only migration apply primitive so a future CLI/repository migrator can execute validated rollback plans without exposing mutation through the shipped daemons or claiming that runtime stores are DB-backed.

The existing planner already produced down steps for explicit target versions, including rollback-to-zero. The previous programmatic apply primitive executed only up steps. This slice closes the matching package boundary: a caller-supplied `database/sql` transaction executor can now run pending down migrations after the same catalog/ledger/target validation.

## Contract frozen by this slice

`db/migrations.ApplyToVersion(...)` and `ApplyCatalogUpToVersion(...)` now execute both directions produced by `PlanToVersion(...)` / `PlanCatalogToVersion(...)`:

- up steps:
  - execute the migration `UpSQL`,
  - then insert the matching `schema_migrations` row with `version`, `name`, and `up_sha256`,
  - update `ApplyResult.current_version` to the applied version.
- down steps:
  - delete the matching `schema_migrations` row by `version`, `name`, and `up_sha256`,
  - then execute the migration `DownSQL`,
  - update `ApplyResult.current_version` to `version - 1`.
- all pending steps still run inside one caller-supplied transaction.
- no-op targets still return without opening a transaction.
- catalog, ledger, target, and checksum validation still run before any transaction is opened.
- ledger inserts/deletes must report exactly one affected row; zero-row, multi-row, nil-result, or unknown-row-count outcomes fail closed and roll back.
- ledger delete failures roll back before executing the down SQL body.
- down SQL failures roll back after the attempted ledger delete.
- rollback-to-zero is supported, including deleting the `0001` ledger row before executing `0001_bootstrap_schema_migrations.down.sql`, which drops `schema_migrations`.
- `ApplyResult.applied` remains metadata-only plan steps and can include `direction = "down"`.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or any other daemon mutation endpoint,
- daemon startup auto-migration,
- a CLI command,
- a production database driver dependency or DB engine selection,
- statement splitting or dialect-specific SQL execution policy,
- backup/restore orchestration around rollback,
- repository writes or DB-backed account/character/item/quest/login-ticket stores.

The shipped `gamed` ops mux remains read-only for migration surfaces: catalog summary, latest status, explicit target plan, ledger-snapshot export, and offline ledger-snapshot planning.

## Why this order

A production rollback command needs backup policy, driver-backed validation, operator prompting, and deployment runbooks. Freezing only the programmatic transaction behavior first keeps the data-safety boundary small while allowing future CLI work to reuse a tested up/down executor rather than inventing rollback semantics at the operator layer.

The explicit down-step order matters for rollback-to-zero: the `schema_migrations` table is itself created by migration `0001`, so the executor must delete the `0001` ledger row before it runs the down SQL that drops the ledger table.

## TDD and validation

Focused coverage added in `db/migrations/apply_test.go` proves:

- rollback plans execute down steps in reverse order in one transaction,
- down steps delete the matching ledger row before running down SQL,
- rollback-to-zero deletes the `0001` ledger row before dropping `schema_migrations`,
- ledger insert/delete row-count drift fails closed and rolls back,
- ledger delete failure rolls back and does not execute down SQL,
- down SQL failure rolls back after the ledger delete attempt,
- existing up-apply, no-op, up failure, ledger insert failure, rollback failure, commit failure, and nil-executor coverage remains green.

Validation run for this slice:

- `go test ./db/migrations -run 'TestApplyCatalogUpToVersion(ExecutesRollback|RollsBackToZero|RollsBackWhenLedgerDelete|RollsBackWhenDown)' -count=1`
- `go test ./db/migrations -count=1`
- full-repo validation is recorded in the cron run output/commit summary.

## Follow-up options

1. Add a driver-backed integration harness for ledger read + apply + rollback before exposing any CLI.
2. Add a CLI-only migration command with explicit backup/restore preflight and a dry-run confirmation flow.
3. Define DB driver/engine support only when a repository or migrator slice needs it.
4. Keep daemon-local ops migration endpoints read-only unless a future production-admin design explicitly changes that boundary.
