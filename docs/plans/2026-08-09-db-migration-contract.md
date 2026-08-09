# DB Migration Contract — 2026-08-09

## Objective

Introduce the first project-owned database migration boundary without claiming that the runtime is DB-backed yet.

The current server still uses bootstrap JSON/file stores for accounts, login tickets, item templates, authored content, and runtime-adjacent QA state. This slice only creates a validated migration catalog and the first schema ledger migration so future repository/backfill work has an explicit durable contract to target.

## Current contract

Migration files live under `db/migrations/` and are loaded by the `db/migrations` Go package.

Rules frozen by tests:

- migration files are flat SQL files named `NNNN_name.up.sql` and `NNNN_name.down.sql`,
- versions are four digits and must be contiguous from `0001`,
- names use lowercase letters, digits, and single underscores,
- each version has exactly one matching `up` and `down` file with the same name,
- SQL bodies must be non-empty UTF-8,
- every file starts with a project-owned header:
  - `-- go-metin2 migration: NNNN name up`,
  - `-- go-metin2 migration: NNNN name down`,
- `migrations.manifest.json` is mandatory and uses format `go-metin2-migration-manifest-v1`,
- the manifest records each migration's version, name, up/down paths, and SHA-256 checksums,
- the loader rejects unknown manifest fields, trailing JSON, duplicate manifest versions, path drift, checksum drift, missing manifest entries, and SQL files that do not match their manifest entry,
- the catalog is returned in deterministic version order and exposes the validated `UpSHA256` / `DownSHA256` values next to the SQL text,
- malformed names, missing pairs, version gaps, mismatched pairs, empty SQL, missing headers, and stale manifest data fail closed with `ErrInvalidCatalog`,
- `PlanUpToLatest` / `PlanCatalogUpToLatest` provide a read-only dry-run boundary that compares a validated catalog against applied `schema_migrations` ledger entries without opening a database or executing SQL,
- the dry-run planner accepts unordered ledger rows but rejects duplicates, gaps, future versions, name drift, checksum drift, zero/negative versions, and any mutated catalog SQL whose body no longer hashes to the manifest-pinned checksum,
- plan steps expose only metadata (`version`, `name`, `direction`, `path`, `sha256`) and intentionally do not expose executable SQL as the plan payload.
- `gamed` exposes a loopback-only read-only `GET /local/db/migrations/status` endpoint that returns the same metadata-only dry-run plan against an empty ledger for now, making the embedded catalog visible to operators without opening a database or executing SQL.
- `ReadSQLLedgerEntries` / `PlanUpToLatestFromSQLLedger` add the first database/sql-compatible ledger reader seam for future preflight tooling: callers supply a `QueryContext` boundary, the package reads only `version`, `name`, and `up_sha256` from `schema_migrations` in version order, closes rows, and fails closed on query, scan, iteration, close, catalog, or ledger drift errors.

The first migration is `0001_bootstrap_schema_migrations` and creates only a minimal `schema_migrations` ledger:

- `version INTEGER PRIMARY KEY`,
- `name TEXT NOT NULL`,
- `up_sha256 TEXT NOT NULL`,
- `applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`.

The `up_sha256` column intentionally pins the exact SQL body that was applied, so a future migrator can refuse to treat a mutated historical migration as already applied.

## What this is not yet

This is not a database runtime implementation. It deliberately does not add:

- a DB driver dependency,
- connection configuration,
- automatic migration apply/rollback commands,
- account/character/item repository implementations,
- JSON snapshot backfill tooling,
- production deployment scripts.

The dry-run planner added on top of the catalog is likewise read-only: callers can supply already-read ledger rows directly or provide a `database/sql`-compatible query boundary for the same metadata through `ReadSQLLedgerEntries` / `PlanUpToLatestFromSQLLedger`. The first loopback ops endpoint deliberately still uses an empty ledger because runtime DB connection configuration is not frozen yet. The SQL ledger seam is a safe boundary for future CLI, real-ledger preflight tooling, or status pages, not an execution engine.

Those require separate slices because each one changes operator and data-safety semantics.

## Likely next slices

1. Add explicit DB connection configuration and wire the local-only migration status preflight to a configured `schema_migrations` reader without mutating a database.
2. Define a narrow account/character repository interface backed by current tests before adding a DB implementation.
3. Add explicit schema migrations for account identity and character roster only after the repository seam is frozen.
4. Add JSON-file-store export/import or quarantine tooling for migration from bootstrap snapshots.
5. Add an apply/rollback command only after the dry-run status boundary and ledger validation behavior are exercised against an actual driver-backed test database.
6. Document production DB configuration, backups, and rollback policy once there is an actual DB-backed store.

## Exit criteria for this slice

- `go test ./db/migrations` validates the catalog, direct-ledger dry-run planning rules, and database/sql-compatible ledger-reader seam.
- `go test ./...` and `go vet ./...` remain green.
- README/development docs describe `db/migrations` as the validated migration catalog and read-only planning skeleton, not a finished DB layer.
