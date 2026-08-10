# DB Migration Contract — 2026-08-09

## Objective

Introduce the first project-owned database migration boundary without claiming that the runtime is DB-backed yet.

The current server still uses bootstrap JSON/file stores for accounts, login tickets, item templates, authored content, and runtime-adjacent QA state. This migration track creates a validated catalog, the schema ledger migration, and the first account/character roster schema contract so future repository/backfill work has an explicit durable boundary to target.

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
- plan steps expose only metadata (`version`, `name`, `direction`, `path`, `sha256`) and intentionally do not expose executable SQL as the plan payload,
- the embedded catalog now includes `0002_account_character_roster`, a schema-only contract for the durable account identity and four-slot character roster boundary:
  - `accounts` stores a project-owned account id, original login, normalized login, empire, and timestamps,
  - `characters` stores a project-owned character id, account id, select-screen slot, original/normalized name, bootstrap appearance/stat/location/guild/gold fields, and timestamps,
  - normalized account logins, `(account_id, slot)`, and normalized character names are unique,
  - the migration deliberately does not add inventory, equipment, quickslots, item instances, quest state, login tickets, static actors, interactions, content bundles, or world runtime tables yet.
- `gamed` exposes a loopback-only read-only `GET /local/db/migrations/status` endpoint that returns the same metadata-only dry-run plan; with no DB config it plans against an empty ledger, and with explicit DB config it reads only the `schema_migrations` ledger before planning.
- `ReadSQLLedgerEntries` / `PlanUpToLatestFromSQLLedger` add the first database/sql-compatible ledger reader seam for future preflight tooling: callers supply a `QueryContext` boundary, the package reads only `version`, `name`, and `up_sha256` from `schema_migrations` in version order, closes rows, and fails closed on query, scan, iteration, close, catalog, or ledger drift errors.
- `PlanToVersion` / `PlanCatalogToVersion` and the matching SQL-ledger variants now provide an explicit target-version dry-run contract:
  - target `latest` is exposed by the existing latest-status path,
  - target `0` previews a complete rollback using down migrations in reverse applied-version order,
  - intermediate targets emit only the up/down steps needed to move from the current ledger version to that target,
  - out-of-range targets fail closed with `ErrInvalidMigrationTarget`,
  - target plans still expose metadata only and never include executable SQL.
- runtime configuration now carries an optional DB preflight boundary:
  - `METIN2_DB_DRIVER` / `METIN2_GAMED_DB_DRIVER`,
  - `METIN2_DB_DSN` / `METIN2_GAMED_DB_DSN`,
  - both empty means DB-backed migration preflight is disabled,
  - partial or malformed values fail startup validation,
  - configured status reads through `database/sql` but does not bundle or select a real driver dependency yet,
  - `/local/runtime-config` reports only `database.configured`, `database.driver`, and `database.dsn_configured`; it never exposes the DSN value.

The first migration is `0001_bootstrap_schema_migrations` and creates only a minimal `schema_migrations` ledger:

- `version INTEGER PRIMARY KEY`,
- `name TEXT NOT NULL`,
- `up_sha256 TEXT NOT NULL`,
- `applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`.

The `up_sha256` column intentionally pins the exact SQL body that was applied, so a future migrator can refuse to treat a mutated historical migration as already applied.

The second migration is `0002_account_character_roster`. It is the first domain schema contract, but it is still a schema-only boundary: the shipped daemons continue to load and save accounts/characters through the bootstrap file store until a future repository/backfill slice replaces that coupling deliberately.

## What this is not yet

This is not a database runtime implementation. It deliberately does not add:

- a DB driver dependency or default production DB engine,
- DB connection pool ownership beyond the read-only migration-status preflight,
- automatic migration apply/rollback commands,
- account/character/item repository implementations or DB-backed runtime writes,
- JSON snapshot backfill tooling,
- production deployment scripts.

The dry-run planner added on top of the catalog is likewise read-only: callers can supply already-read ledger rows directly or provide a `database/sql`-compatible query boundary for the same metadata through `ReadSQLLedgerEntries` / `PlanUpToLatestFromSQLLedger` / `PlanToVersionFromSQLLedger`. The first loopback ops endpoints use an empty ledger when DB config is disabled and a configured `database/sql` ledger reader when both driver and DSN are set. `/local/db/migrations/status` reports the latest-version target; `/local/db/migrations/plan?target_version=N` previews an explicit target such as rollback-to-zero. The SQL ledger seam and runtime config are safe boundaries for future CLI, real-ledger preflight tooling, or status pages, not an execution engine.

Those require separate slices because each one changes operator and data-safety semantics.

## Likely next slices

1. Define a narrow account/character repository interface backed by current tests before adding a DB implementation.
2. Add JSON-file-store export/import or quarantine tooling that can map bootstrap account snapshots into the `0002_account_character_roster` shape without silently coercing bad snapshots.
3. Add a driver-backed test harness or build-tagged integration test for `schema_migrations` status before adding apply/rollback tooling.
4. Add explicit migrations for inventory/equipment/quickslots only after the account/character repository seam is stable.
5. Add an apply/rollback command only after the dry-run status boundary and ledger validation behavior are exercised against an actual driver-backed test database.
6. Document production DB configuration, backups, and rollback policy once there is an actual DB-backed store.

## Exit criteria for this slice

- `go test ./db/migrations` validates the catalog, schema ledger migration, account/character roster migration, direct-ledger dry-run planning rules, explicit up/down target planning, and database/sql-compatible ledger-reader seam.
- `go test ./internal/config ./internal/minimal ./internal/service ./internal/ops` validates optional DB config loading, startup fail-closed behavior for partial config, no-DSN runtime-config exposure, the configured-driver migration-status boundary, and loopback-only explicit migration-plan previews.
- `go test ./...` and `go vet ./...` remain green.
- README/development docs describe `db/migrations` as the validated migration catalog and read-only planning skeleton, not a finished DB layer.
