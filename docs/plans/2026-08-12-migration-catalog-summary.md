# Migration Catalog Summary Endpoint — 2026-08-12

## Objective

Expose the validated project-owned migration catalog as a metadata-only runtime/operator surface before adding any migration executor or DB-backed repository implementation.

Operators already have dry-run status and offline ledger-snapshot planning endpoints. This slice adds the missing catalog-inventory boundary: what exact migration files and checksums does this shipped `gamed` binary embed?

## Contract frozen by this slice

`db/migrations` now exposes:

- `CatalogSummaryFormat = go-metin2-migration-catalog-summary-v1`,
- `BuiltInCatalogSummary()`,
- `CatalogSummary(catalog)`.

The summary shape is deliberately metadata-only:

- `format`,
- `latest_version`,
- deterministic `migrations` rows with:
  - `version`,
  - `name`,
  - `up_path`,
  - `down_path`,
  - `up_sha256`,
  - `down_sha256`.

The helper validates the catalog before returning a summary, preserves the embedded catalog order, pins both up/down checksums, and never includes executable SQL text.

`gamed` now registers loopback-only read-only:

- `GET /local/db/migrations/catalog`

The endpoint:

- is registered only on `gamed`,
- rejects non-`GET` methods with `405`,
- rejects non-loopback callers with `403`,
- returns `409` if embedded catalog validation fails,
- does not open or inspect the configured DB,
- does not expose DSNs, applied ledger rows, runtime store data, `CREATE TABLE` / `DROP TABLE` SQL, or apply/rollback output.

## Why this belongs before apply/rollback

Dry-run plan endpoints answer “what would this daemon do against this ledger?” The catalog summary answers the simpler preflight question “what migration inventory does this daemon carry?” That gives operators and future CLI/runbook tooling a stable checksum manifest to compare before applying migrations or importing JSON projections.

## What this is not yet

This slice does not add:

- migration execution,
- rollback execution,
- DB driver selection,
- DB-backed account/item/login-ticket repositories,
- JSON export import/backfill tooling,
- production deployment automation.

## TDD and validation

Primary focused coverage:

- `go test ./db/migrations -run TestCatalogSummary -count=1`,
- `go test ./internal/minimal -run TestGameRuntimeMigrationCatalogSummary -count=1`,
- `go test ./internal/ops -run TestLocalMigrationCatalog -count=1`.

Full validation remains `go test ./...`, `go vet ./...`, `gofmt -l`, and `git diff --check` before commit/push.
