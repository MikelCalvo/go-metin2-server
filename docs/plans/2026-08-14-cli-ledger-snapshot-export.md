# CLI Ledger-Snapshot Export Boundary — 2026-08-14

## Objective

Add a read-only `metin2-migrate ledger-snapshot` command so operators can export strict `schema_migrations` metadata from a `database/sql` target without relying on a running `gamed` local ops endpoint and without applying migrations.

The repo already had the strict `go-metin2-schema-migrations-ledger-v1` snapshot shape, loopback-only daemon export endpoint, offline snapshot planning, and CLI-only apply boundary. This slice closed the CLI preflight loop by letting the same CLI produce the snapshot artifact that `metin2-migrate plan` and `metin2-migrate apply` consume. A later small helper, `metin2-migrate empty-ledger-snapshot`, now emits the strict version-zero artifact for first-time database initialization without hand-written JSON.

## Contract frozen by this slice

`metin2-migrate ledger-snapshot` accepts:

```bash
metin2-migrate ledger-snapshot \
  --driver <database/sql-driver> \
  --dsn <dsn>
```

Behavior:

- requires both `--driver` and `--dsn`;
- opens the operator-supplied `database/sql` target only long enough to query `schema_migrations` metadata;
- reads only `version`, `name`, and `up_sha256` through the existing `db/migrations.LedgerSnapshotFromSQLLedger(...)` helper;
- writes strict `go-metin2-schema-migrations-ledger-v1` JSON to stdout;
- never emits executable SQL text, runtime store payloads, apply results, or the supplied DSN;
- redacts the supplied DSN from runtime errors before printing stderr;
- performs no transaction begin, SQL exec, migration apply, rollback, or daemon mutation.

Exit-code policy stays aligned with the rest of the CLI:

- `0` = success;
- `1` = runtime/validation failure such as unknown driver, query failure, row scan failure, or invalid ledger metadata;
- `2` = usage error such as missing driver/DSN or extra positional arguments.

## What this is not yet

This slice deliberately does not add:

- a bundled production database driver or default database engine;
- daemon startup auto-migration;
- daemon-local migration mutation endpoints;
- backup/restore orchestration around mutating migration runs;
- advisory locks or multi-operator coordination;
- DB-backed account, character, item, quest, content, or login-ticket repositories.

The shipped daemon-local migration endpoints remain read-only. `metin2-migrate ledger-snapshot` is also read-only, but unlike `plan` it does open the operator-supplied DB target for one metadata query.

## Why this order

The previous CLI apply boundary required an offline ledger snapshot. Before production runbooks can recommend that flow, operators need a CLI-only way to create the snapshot artifact in environments where the daemon is not running or where local ops access is intentionally unavailable. Exporting only the ledger metadata keeps the command safe while making the existing plan/apply workflow more complete.

## TDD and validation

Focused coverage in `internal/migratecli/migratecli_test.go` proves:

- `ledger-snapshot` exports strict metadata-only snapshot JSON from a registered test driver;
- the command performs only an open/query/close path and does not begin a transaction, execute SQL, commit, or roll back;
- missing driver/DSN fails as usage;
- runtime errors redact the supplied DSN.

Validation for this slice:

- `go test ./internal/migratecli -run 'TestRunLedgerSnapshot' -count=1`;
- `go test ./internal/migratecli ./db/migrations -count=1`;
- `go test ./... -count=1 -timeout=120s`;
- `go vet ./...`;
- `gofmt -l .`;
- `git diff --check`.

## Follow-up options

1. Add a build-tagged driver-backed integration harness after selecting a concrete DB engine.
2. Add a migration runbook that sequences `ledger-snapshot -> plan -> backup/preflight -> apply` once backup/restore orchestration is frozen.
3. Add advisory locking or single-writer coordination after deployment topology and DB engine are known.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
