# DB Driver Startup Preflight — 2026-08-13

## Objective

Fail closed at daemon startup when an operator configures a DB migration-preflight driver that is not actually linked into the binary.

The migration status and ledger-snapshot endpoints are still metadata-only and read-only, but a configured driver name is an explicit production-ops intent. A daemon should not start successfully with `METIN2_*_DB_DRIVER` / `METIN2_*_DB_DSN` set and then make every DB migration preflight fail later because `database/sql` has no registered driver.

## Contract frozen by this slice

`internal/config` now exposes `ValidateDatabaseDriverAvailability(cfg)`:

- disabled DB preflight (`driver == ""` and `dsn == ""`) is accepted,
- partial or malformed DB config still fails with the existing `ValidateDatabaseConfig` errors,
- a non-empty configured driver must be present in `database/sql`'s registered driver list,
- an unavailable driver fails with `ErrDatabaseDriverUnavailable`,
- the helper validates only driver availability and never opens a DSN, pings a database, queries `schema_migrations`, or applies migration SQL.

Startup wiring now uses this stricter preflight for the service runtime and the shipped `gamed` runtime path before starting the ops listener or legacy TCP listener. This keeps the current stock binary honest: because the project does not yet ship or select a real DB driver dependency, operators should leave DB preflight disabled unless they build a binary that registers an appropriate driver.

The lower-level migration-status helpers still return `ErrDatabaseDriverUnavailable` if called directly with an unavailable driver, so existing runtime tests and future injected-driver tests can exercise that path without starting a daemon.

## What this is not yet

This slice does not add:

- a production DB driver dependency,
- DB engine selection,
- connection-pool ownership beyond existing read-only preflights,
- driver-backed integration tests,
- migration apply/rollback tooling,
- DB-backed account, character, item, quest, content, or login-ticket repositories.

## TDD and validation

Focused coverage:

- `go test ./internal/config -run 'TestValidateDatabaseDriverAvailabilityRejectsUnknownConfiguredDriver|TestValidateDatabaseConfig' -count=1`,
- `go test ./internal/service -run 'TestRunRejectsUnavailableDatabaseDriverBeforeStartingLegacyServer|TestRunRejectsPartialDatabaseConfigBeforeStartingLegacyServer' -count=1`,
- `go test ./internal/minimal -run 'TestNewGameRuntimeRejectsUnavailableDatabaseDriver|TestNewGameRuntimeRejectsPartialDatabaseConfig|TestGameRuntimeMigrationStatusRejectsConfiguredDatabaseWithoutRegisteredDriver|TestGameRuntimeMigrationLedgerSnapshotRejectsConfiguredDatabaseWithoutRegisteredDriver' -count=1`.

Full validation remains `gofmt -l`, `go test ./... -count=1 -timeout=120s`, `go vet ./...`, and `git diff --check` before commit/push.

## Follow-up options

1. ~~Add a build-tagged or injected-driver test harness for a real `schema_migrations` status read.~~ Done for the build-tagged SQLite harness (`go test -tags=sqlite_harness ./db/migrations -run SQLiteHarness`) that applies the catalog on a temp DB and proves live ledger/status/snapshot round-trips — see [SQLite schema_migrations driver-backed harness](2026-08-25-sqlite-schema-migrations-driver-harness.md). Stock binaries still do not register/link a production driver.
2. Choose and document a production DB driver/engine only when repository or migrator work needs it.
3. ~~Keep apply/rollback commands out of scope until driver-backed preflight and recovery semantics are proven.~~ The programmatic apply primitive and CLI `apply` already exist; keep production-engine selection / SQL import-backfill deferred until operators choose an engine on top of the harness gate.
