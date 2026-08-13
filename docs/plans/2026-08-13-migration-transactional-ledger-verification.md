# Migration Transactional Ledger Verification — 2026-08-13

## Objective

Harden the package-only migration apply/rollback primitive so a future CLI or repository migrator cannot execute a mutating plan solely from a stale caller-supplied ledger snapshot.

The previous apply primitive validated the supplied ledger before opening a transaction, then executed the pending up/down steps and wrote or deleted `schema_migrations` rows. That was enough for deterministic unit execution, but it did not prove that the transaction-local database ledger still matched the caller's preflight expectation or that the ledger matched the requested target immediately before commit.

## Contract frozen by this slice

`db/migrations.ApplyToVersion(...)` and `ApplyCatalogUpToVersion(...)` now add transaction-local ledger verification around mutating plans:

- The caller-supplied ledger is still validated against the catalog before any transaction opens.
- No-op plans still return without opening a transaction.
- For mutating plans whose current version is greater than zero, the executor reads `schema_migrations` inside the opened transaction before executing migration SQL.
  - Those rows must validate against the same catalog.
  - They must match the caller-supplied ledger boundary.
  - Drift fails closed and rolls the transaction back before any migration SQL is executed.
- Rolling an empty database up from version zero deliberately skips the pre-read because `schema_migrations` does not exist until migration `0001` runs.
- For any non-zero target version, the executor reads `schema_migrations` again after applying all pending steps and before commit.
  - The rows must validate against the catalog.
  - They must match the expected contiguous ledger for the target version.
  - Drift fails closed and rolls the transaction back instead of committing a partially applied batch.
- Rollback-to-zero still deletes the `0001` ledger row before running `0001_bootstrap_schema_migrations.down.sql`; it deliberately skips the post-read because that down migration drops `schema_migrations`.
- Ledger inserts/deletes still must report exactly one affected row.
- Migration SQL, ledger write/delete, ledger read, row-count, commit, and rollback failures remain surfaced with explicit errors and no daemon mutation endpoint is added.

## What this is not yet

This slice deliberately does not add:

- `/local/db/migrations/apply`, `/local/db/migrations/rollback`, or another daemon mutation endpoint,
- daemon startup auto-migration,
- a CLI command,
- production database driver selection or a new DB dependency,
- advisory locks or cross-process migration coordination,
- backup/restore orchestration around a migration run,
- DB-backed account/character/item/quest/login-ticket repositories.

The shipped `gamed` ops migration endpoints remain read-only: catalog summary, latest status, explicit target plan, ledger-snapshot export, and offline ledger-snapshot planning.

## Why this order

Before a real migrator or CLI can safely apply schema changes, the transaction boundary must protect against stale preflight input. Re-reading the ledger inside the same transaction gives a future caller a small, tested guardrail without choosing a database engine, lock primitive, deployment flow, or public operator surface too early.

The special cases are intentional:

- version-zero apply cannot query a ledger table that migration `0001` has not created yet,
- rollback-to-zero cannot query a ledger table after migration `0001` drops it.

Those edges are now explicit instead of accidental.

## TDD and validation

Focused coverage added/updated in `db/migrations/apply_test.go` proves:

- normal up-apply records ledger rows and verifies the post-apply ledger before commit,
- stale transaction-local ledger state rolls back before migration SQL executes,
- post-apply ledger drift rolls back before commit,
- transaction-local ledger read failures roll back,
- rollback plans verify the pre-apply ledger and the post-rollback target ledger,
- rollback-to-zero still deletes the `0001` ledger row before dropping `schema_migrations`,
- existing row-count, SQL-failure, rollback-failure, commit-failure, and nil-executor guard coverage remains green.

Validation run for this slice:

- `go test ./db/migrations -run 'TestApplyCatalogUpToVersion(ExecutesPending|RejectsStale|RejectsLedgerDrift|RollsBackWhenTransactionalLedgerRead)' -count=1`
- `go test ./db/migrations -count=1`
- full-repo validation is recorded in the cron run output/commit summary.

## Follow-up options

1. Add a CLI-only migrator that uses the existing dry-run plan, ledger snapshot export, and transactional apply primitive.
2. Add a driver-backed integration harness once the project selects a concrete DB engine for production migrations.
3. Add advisory-lock or single-writer migration coordination after a DB engine and deployment topology are chosen.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design explicitly changes that boundary.
