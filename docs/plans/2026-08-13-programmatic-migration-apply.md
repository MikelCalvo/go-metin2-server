# Programmatic Migration Apply Primitive — 2026-08-13

## Objective

Add the first test-backed migration apply boundary without exposing migration execution through the shipped daemons or claiming that runtime stores are DB-backed.

The existing DB lane already had a validated catalog, dry-run target planner, SQL ledger reader, strict offline ledger snapshots, and local read-only ops endpoints. This slice adds the next narrow primitive for future CLI/repository work: applying pending **up** migrations inside a caller-supplied database transaction boundary.

## Contract frozen by this slice

`db/migrations` now exposes:

- `SQLMigrationExecutor`
  - a narrow `database/sql` transaction boundary with `BeginTx(ctx, opts)`,
  - satisfied by `*sql.DB` and `*sql.Conn`,
  - intentionally does not own driver selection, DSN loading, connection pools, or daemon startup policy.
- `ApplyResult`
  - `previous_version`,
  - `current_version`,
  - `latest_version`,
  - `applied` plan steps.
- `ApplyUpToLatest(...)`, `ApplyToVersion(...)`, `ApplyCatalogUpToLatest(...)`, and `ApplyCatalogUpToVersion(...)`.

Behavior:

- validates the catalog/ledger/target using the same planner boundary as dry-run status,
- returns a no-op result without opening a transaction when the requested target is already applied,
- supports only pending `up` steps,
- rejects rollback/down targets with `ErrMigrationApplyUnsupportedDirection` before opening a transaction,
- rejects nil or typed-nil executors with `ErrMigrationApplyExecutorRequired`,
- runs all pending up migrations and their ledger inserts in one transaction,
- executes each migration SQL body before inserting the matching `schema_migrations` row,
- writes ledger rows with `version`, `name`, and `up_sha256`, matching the `0001_bootstrap_schema_migrations` ledger contract,
- rolls back when migration SQL or ledger insertion fails,
- joins rollback failure with the original apply failure when rollback itself fails,
- reports commit failures without hiding them,
- returns only metadata; it does not expose executable SQL, DSNs, connection details, row data, or runtime store contents.

## What this is not yet

This slice deliberately does not add:

- an HTTP/local ops apply endpoint,
- daemon startup auto-migration,
- a CLI command,
- rollback/down execution,
- a production database driver dependency,
- DB engine selection,
- statement splitting or dialect-specific SQL execution policy,
- DB-backed account, character, item, quest, content, or login-ticket repositories,
- JSON snapshot import/backfill execution.

The shipped `gamed` ops mux remains read-only for migration surfaces: catalog summary, latest status, explicit target plan, ledger-snapshot export, and offline ledger-snapshot planning.

## Why this order

A production migration command needs stronger recovery and deployment policy than a package-level primitive. Freezing the transaction boundary first lets future CLI or repository work reuse a small tested apply contract while keeping daemon-local ops endpoints safe and read-only.

## TDD and validation

Focused coverage added in `db/migrations/apply_test.go` proves:

- pending migrations are executed and ledgered in one transaction,
- no-op targets do not open a transaction,
- rollback targets fail before transaction begin,
- migration SQL failure rolls back before writing that migration's ledger row,
- ledger insert failure rolls back,
- rollback failure is reported with the original error,
- commit failure is reported,
- nil and typed-nil executors fail closed.

Validation run for this slice:

- `go test ./db/migrations -count=1`
- `go test ./... -count=1 -timeout=120s`
- `go vet ./...`
- `gofmt -l .`
- `git diff --check`

## Follow-up options

1. Add a driver-backed integration harness for the ledger reader and apply primitive before exposing any command.
2. Choose a production DB driver/engine only when a repository or migrator slice needs it.
3. Add a CLI-only migration apply command with explicit backup/restore preflight policy.
4. Add rollback/down execution as a separate slice with tested recovery semantics.
5. Keep daemon-local ops migration endpoints read-only unless a future production-admin design explicitly changes that boundary.
