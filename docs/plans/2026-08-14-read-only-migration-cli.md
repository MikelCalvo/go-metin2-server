# Read-only Migration CLI Preflight — 2026-08-14

## Objective

Add the first shipped migration command surface without exposing database mutation through daemons or pretending the runtime is DB-backed.

The migration package already had a validated embedded catalog, strict offline ledger snapshots, read-only local ops preflights, and a package-only transactional up/down executor. This slice adds the safe operator bridge before any production apply/rollback command: a tiny CLI that can print catalog metadata and dry-run a target plan from an offline ledger snapshot.

## Contract frozen by this slice

`cmd/metin2-migrate` now ships with two read-only commands:

- `metin2-migrate catalog`
  - validates the embedded catalog,
  - prints `go-metin2-migration-catalog-summary-v1` JSON,
  - includes version, name, paths, and SHA-256 checksums only,
  - never prints executable SQL.
- `metin2-migrate plan --ledger-snapshot <path|-> --target-version <version|latest>`
  - reads strict `go-metin2-schema-migrations-ledger-v1` JSON from a file or stdin,
  - bounds snapshot input at 64 KiB before planning,
  - validates the snapshot against the embedded catalog,
  - prints the metadata-only dry-run plan toward the requested target version,
  - supports both up plans and rollback plans, including target `0`,
  - accepts `latest` as an alias for the embedded catalog tip,
  - never opens a database and never applies or rolls back SQL.

Exit-code policy:

- `0` = success,
- `1` = validation/runtime failure such as an invalid ledger snapshot or catalog/ledger drift,
- `2` = usage error such as an unknown command or missing flags.

`internal/migratecli.Run(...)` owns the testable command behavior; `cmd/metin2-migrate` only adapts process args/stdin/stdout/stderr. `make build`, CI, and the Docker build stage now include the CLI alongside `authd` and `gamed`.

## What this is not yet

This slice deliberately does not add:

- migration apply/rollback CLI commands,
- daemon startup auto-migration,
- `/local/db/migrations/apply` or another mutation endpoint,
- a production database driver dependency or DB engine selection,
- DB connection opening from the CLI,
- backup/restore orchestration around mutating migration runs,
- DB-backed account/character/item/quest/login-ticket repositories.

The shipped daemon migration endpoints remain read-only, and this CLI remains read-only too.

## Why this order

The project needs a practical operator artifact before production migration execution, but mutation still requires recovery policy, a concrete driver/engine, and backup/restore runbooks. A read-only CLI lets maintainers test catalog and ledger-snapshot workflows in CI/local shells while preserving the current fail-closed boundary: any migration execution still goes only through the package primitive used by tests and future tools.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `catalog` writes metadata-only catalog JSON and omits SQL,
- `plan` reads an offline ledger snapshot from stdin or a file path and emits a target plan,
- invalid or oversized ledger snapshots fail before writing a plan,
- `latest` resolves to the embedded catalog tip,
- unsupported mutating commands such as `apply` fail as usage errors.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRun' -count=1`,
- `go test ./internal/migratecli ./db/migrations -count=1`,
- `go test ./... -count=1 -timeout=120s`,
- `go vet ./...`,
- `gofmt -l .`,
- `git diff --check`.

## Follow-up options

1. Add a driver-backed integration harness once the project selects a concrete DB engine.
2. Add a backup/restore preflight contract for mutating migration runs.
3. Add explicit CLI apply/rollback commands only after the driver and recovery policy are proven.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.