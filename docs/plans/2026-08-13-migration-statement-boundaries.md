# Migration Statement Boundaries — 2026-08-13

## Objective

Harden the package-only migration executor so future CLI/repository migrators can run multi-statement migration files through `database/sql` without relying on a driver accepting an entire SQL file as one `ExecContext` call.

The existing apply/rollback primitive already validated catalog metadata, ledger checksums, transactional row counts, and transaction-local ledger drift. This slice keeps that boundary package-only and adds the missing statement execution contract.

## Contract frozen by this slice

`db/migrations` now validates conservative SQL statement boundaries as part of catalog/planner validation:

- every migration SQL body must contain at least one terminated executable statement,
- executable trailing SQL without a semicolon fails closed with `ErrInvalidCatalog`,
- unterminated single-quoted strings, double-quoted identifiers, and block comments fail closed,
- semicolons inside single-quoted strings, double-quoted identifiers, line comments, or block comments do not split statements,
- doubled quotes inside quoted strings/identifiers are preserved,
- statement text is trimmed and still includes relevant comments/header text attached to that statement.

`ApplyCatalogUpToVersion(...)` now executes migration bodies statement-by-statement inside the same caller-supplied transaction:

- up migrations execute all split up statements before the matching `schema_migrations` ledger insert,
- down migrations delete the matching `schema_migrations` ledger row before executing all split down statements,
- multi-statement migrations still roll back as one transaction on SQL, ledger, row-count, verification, or commit failure,
- no-op targets still return without opening a transaction,
- invalid statement boundaries are rejected before a transaction is opened.

## What this is not yet

This slice deliberately does not add:

- a SQL dialect parser,
- PostgreSQL dollar-quoted strings or dialect-specific procedural bodies,
- statement parameter binding,
- DB driver selection or a new driver dependency,
- CLI migration commands,
- daemon startup auto-migration,
- `/local/db/migrations/apply` or another daemon mutation endpoint,
- DB-backed account/character/item/quest/login-ticket repositories.

The shipped `gamed` migration ops endpoints remain read-only: catalog summary, status, target plan, ledger snapshot export, and offline ledger-snapshot planning.

## Why this order

The migration catalog already contains multi-statement files. Executing the whole file as one `ExecContext` call is too driver-dependent for future production tooling, but selecting a production database engine or building a CLI would still be premature. A small conservative splitter lets the existing executor behave predictably while preserving the current package-only boundary.

## TDD and validation

Focused coverage now proves:

- quoted semicolons and escaped quotes are preserved,
- comment semicolons do not split statements,
- catalog loading rejects missing terminating semicolons,
- catalog loading rejects unterminated quoted SQL,
- the apply primitive executes every statement before the ledger insert,
- the apply primitive does not execute a combined multi-statement body as one driver call,
- invalid statement boundaries fail before a transaction is opened,
- existing rollback ordering and transaction-local ledger verification remain green.

Validation run for this slice:

- `go test ./db/migrations -count=1`
- `go test ./... -count=1 -timeout=120s`
- `go vet ./...`
- `gofmt -l .`
- `git diff --check`

## Follow-up options

1. Add a CLI-only migration command that uses the existing dry-run plan, ledger snapshot export, and transactional statement executor.
2. Add a driver-backed integration harness once the project selects a concrete DB engine for production migrations.
3. Add backup/restore preflight orchestration around mutating migration commands before exposing them to operators.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design explicitly changes that boundary.
