# Bootstrap ground-item-state SQL import/backfill — 2026-08-26

## Objective

Add the sixth programmatic SQL import/backfill seam for quarantined
`0010_bootstrap_ground_item_state` exports: validate through the existing
quarantine contract, insert rows into `bootstrap_ground_items` inside one
transaction (item-shaped and gold-shaped XOR rows), and prove the round-trip on
the build-tagged SQLite harness after parent `0002` roster rows exist.

This extends Track E SQL import beyond the landed `0002` roster, `0003`
item-state, `0004` quest-state, `0011` point-state, and `0014`/`0015` safebox
seams — without selecting a production engine, shipping a stock release driver,
wiring a daemon/ops mutation endpoint, inventing upsert/idempotent rewrite
policy, or claiming DB-backed live ground rematerialize.

## Why now

- `accountstore.ImportAccountCharacterRoster` already owns the FK parents
  (`accounts` / `characters`) under the SQLite harness.
- Export + quarantine for `0010` ground-item-state are green offline and on
  loopback; the missing cohesive step is INSERT execution against a real engine
  for the pending ground handles the PvE kill → drop → reconnect path already
  rematerializes through the dedicated ground-item FileStore.
- Track E and migration-contract follow-ups still name `0010` SQL import as
  deferred after `0015`. Owning ground next closes the last migration-shaped
  child import named beside the landed FileStore rematerialize path.

## Contract frozen by this slice

1. New primitive lives in `internal/worldruntime`:
   - `ImportBootstrapGroundItemState(ctx, executor, export) (BootstrapGroundItemStateImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineBootstrapGroundItemStateExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger entry for version `10` /
     `bootstrap_ground_item_state` (empty/missing ledger or tip `< 10` fail closed);
   - inserts ground rows with parameterized `INSERT` statements (no `OR REPLACE`
     / upsert) using durable columns only:
     - `vid`, `vnum`, `item_count`, `gold_amount`, `owner_login`,
       `owner_character_id`, `owner_vid`, `owner_name`, `map_index`, `x`, `y`,
       `z`, `pickup_range`
     - omit `created_at` / `updated_at` (database defaults);
     - bind both `item_count` and `gold_amount` every row: the present shape
       uses `int64(*ptr)`, the absent shape binds SQL `NULL` (never `0`);
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name` (`10` / `bootstrap_ground_item_state`);
   - `ground_item_count` / `item_shaped_count` / `gold_shaped_count`;
   - sorted `vids`.
   It never includes ground payloads, SQL text, DSNs, or FileStore snapshot bytes.
4. Empty quarantined exports (present empty `ground_items` slice) succeed as a
   no-op transaction after the ledger gate (counts `0`).
5. Duplicate primary keys / unique-index collisions fail closed and roll the
   transaction back (no partial import).
6. Missing parent `characters` rows fail closed via FK (or equivalent engine
   error) and roll the transaction back — callers must import `0002` roster
   first (or otherwise ensure parents exist).
7. The primitive does **not**:
   - mutate bootstrap FileStores / MemoryStores / live shared-world handles;
   - change `schema_migrations` rows;
   - open a DSN itself or register a production driver;
   - expose a `gamed` / `authd` ops mutation route;
   - invent upsert / merge / truncate-and-reload policy;
   - claim DB-backed runtime rematerialize (FileStore remains the restart path).
8. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0010` on a temp SQLite DB, imports a quarantined sample roster then
   ground-item-state export (item-shaped + gold-shaped rows), and `SELECT`s the
   durable rows back (nullable XOR columns via `sql.NullInt64`). Default
   untagged `go test ./...` stays free of the SQLite dependency.
9. Docs mark Track E / migration-contract SQL-import follow-ups as owned for
   `0002` + `0003` + `0004` + `0010` + `0011` + `0014`/`0015`; CLI wiring,
   production-engine selection, upsert policy, and scheduled purge fold remain
   deferred.

## What this is not yet

- `metin2-migrate` CLI import command
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for live ground rematerialize
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/worldruntime/ground_item_state_import.go` (new)
- `internal/worldruntime/ground_item_state_import_test.go` (new; untagged fail-closed cases)
- `internal/worldruntime/ground_item_state_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic / no BeginTx
- sqlite harness: apply to `>= 10` → import roster parents → import sample
  ground state → SELECT matches item-shaped / gold-shaped / NULL XOR columns
- sqlite harness: second import of the same vids fails closed (unique conflict)
- sqlite harness: import before migrations / without `0010` fails closed
- sqlite harness: import without parent character rows fails closed (FK)
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/worldruntime -run 'BootstrapGroundItemStateImport' -count=1
go test -tags=sqlite_harness ./internal/worldruntime -run 'SQLiteHarness.*GroundItemStateImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic `0010` import primitive is green under the SQLite harness
- Track E docs name `0002`+`0003`+`0004`+`0010`+`0011`+`0014`/`0015` SQL import
  as owned and keep CLI / engine selection / upsert deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
