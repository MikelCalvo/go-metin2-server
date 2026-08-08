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
- the catalog is returned in deterministic version order,
- malformed names, missing pairs, version gaps, mismatched pairs, empty SQL, and missing headers fail closed with `ErrInvalidCatalog`.

The first migration is `0001_bootstrap_schema_migrations` and creates only a minimal `schema_migrations` ledger:

- `version INTEGER PRIMARY KEY`,
- `name TEXT NOT NULL`,
- `applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`.

## What this is not yet

This is not a database runtime implementation. It deliberately does not add:

- a DB driver dependency,
- connection configuration,
- automatic migration apply/rollback commands,
- account/character/item repository implementations,
- JSON snapshot backfill tooling,
- production deployment scripts.

Those require separate slices because each one changes operator and data-safety semantics.

## Likely next slices

1. Add a migration status/apply dry-run CLI or local-only preflight that reads this catalog without mutating a database.
2. Define a narrow account/character repository interface backed by current tests before adding a DB implementation.
3. Add explicit schema migrations for account identity and character roster only after the repository seam is frozen.
4. Add JSON-file-store export/import or quarantine tooling for migration from bootstrap snapshots.
5. Document production DB configuration, backups, and rollback policy once there is an actual DB-backed store.

## Exit criteria for this slice

- `go test ./db/migrations` validates the catalog rules.
- `go test ./...` and `go vet ./...` remain green.
- README/development docs describe `db/migrations` as the validated migration catalog skeleton, not a finished DB layer.
