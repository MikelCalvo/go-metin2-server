# Account/character roster SQL import/backfill — 2026-08-25

## Objective

Add the first programmatic SQL import/backfill seam for quarantined
`0002_account_character_roster` exports: validate through the existing quarantine
contract, insert rows into `accounts` / `characters` inside one transaction, and
prove the round-trip on the build-tagged SQLite harness.

This closes the Track E unlock that was deferred until a driver-backed harness
existed — without selecting a production engine, shipping a stock release
driver, wiring a daemon/ops mutation endpoint, or inventing upsert/idempotent
rewrite policy.

## Why now

- The build-tagged SQLite harness already applies the embedded catalog and
  proves live `schema_migrations` ledger/status/snapshot round-trips.
- Export + quarantine for tip-`0015` kinds (including `0002`) are green offline
  and on loopback; the missing cohesive step is INSERT execution against a real
  engine for the first domain schema.
- Track E item 1 and migration-contract follow-up #4 still name SQL
  import/backfill as deferred. Starting with roster (`0002`) keeps FK parents
  honest before item/quest/safebox child imports.

## Contract frozen by this slice

1. New primitive lives in `internal/accountstore`:
   - `ImportAccountCharacterRoster(ctx, executor, export) (AccountCharacterRosterImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineAccountCharacterRosterExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger version `>= 2` that includes
     `account_character_roster` (empty/missing ledger or tip `< 2` fail closed);
   - inserts accounts then characters with parameterized `INSERT` statements
     (no `OR REPLACE` / upsert);
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name` (`2` / `account_character_roster`);
   - `account_count` / `character_count`;
   - sorted `account_ids` / `character_ids`.
   It never includes passwords, SQL text, DSNs, or account snapshot bytes.
4. Empty quarantined exports (present empty slices) succeed as a no-op
   transaction after the ledger gate (counts `0`).
5. Duplicate primary keys / unique-index collisions fail closed and roll the
   transaction back (no partial import).
6. The primitive does **not**:
   - mutate bootstrap FileStores / MemoryStores;
   - change `schema_migrations` rows;
   - open a DSN itself or register a production driver;
   - expose a `gamed` / `authd` ops mutation route;
   - import `0003+` domains.
7. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0002` on a temp SQLite DB, imports a quarantined sample roster, and
   `SELECT`s the durable rows back. Default untagged `go test ./...` stays free
   of the SQLite dependency.
8. Docs mark Track E / migration-contract SQL-import follow-ups as started for
   `0002` only; `0003+` imports, CLI wiring, production-engine selection, and
   scheduled purge fold remain deferred.

## What this is not yet

- `metin2-migrate` CLI import command
- SQL import for `0003` item-state / `0004` quest / `0010` ground / `0014` /
  `0015` safebox (or other tip kinds)
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for login/select
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/accountstore/roster_import.go` (new)
- `internal/accountstore/roster_import_test.go` (new; untagged fail-closed cases)
- `internal/accountstore/roster_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (optional one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic
- sqlite harness: empty DB apply to `>= 2` → import sample → SELECT matches
- sqlite harness: second import of the same ids fails closed (unique conflict)
- sqlite harness: import before migrations / at ledger tip `0` fails closed
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/accountstore -run 'AccountCharacterRosterImport' -count=1
go test -tags=sqlite_harness ./internal/accountstore -run 'SQLiteHarness.*RosterImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic `0002` import primitive is green under the SQLite harness
- Track E docs name `0002` SQL import as owned and keep `0003+` / CLI / engine
  selection deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
