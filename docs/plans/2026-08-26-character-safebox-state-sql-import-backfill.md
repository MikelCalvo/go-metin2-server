# Character safebox-state SQL import/backfill — 2026-08-26

## Objective

Add the fifth programmatic SQL import/backfill seam for quarantined
`0015_character_safebox_money` exports: validate through the existing quarantine
contract, insert rows into `character_safebox_passwords` /
`character_safebox_items` inside one transaction (including the additive
warehouse `money` column from `0015`), and prove the round-trip on the
build-tagged SQLite harness after parent `0002` roster rows exist.

This extends Track E SQL import beyond the landed `0002` roster, `0003`
item-state, `0004` quest-state, and `0011` point-state seams — without selecting
a production engine, shipping a stock release driver, wiring a daemon/ops
mutation endpoint, inventing upsert/idempotent rewrite policy, or importing
`0010` ground / other tip domains.

## Why now

- `accountstore.ImportAccountCharacterRoster` already owns the FK parents
  (`accounts` / `characters`) under the SQLite harness.
- Export + quarantine for `0014`/`0015` safebox-state are green offline and on
  loopback; the missing cohesive step is INSERT execution against a real engine
  for the durable warehouse cells + money that the PvE restart rematerialize
  path already persists in the safebox FileStore.
- Track E and migration-contract follow-ups still name `0010` / `0014` /
  `0015` SQL import as deferred after `0011`. Owning safebox next keeps the
  warehouse durability path honest before ground-handle child import.

## Contract frozen by this slice

1. New primitive lives in `internal/safeboxstore`:
   - `ImportCharacterSafeboxState(ctx, executor, export) (CharacterSafeboxStateImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineCharacterSafeboxStateExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger entry for version `15` /
     `character_safebox_money` (empty/missing ledger or tip `< 15` fail closed);
   - inserts password rows (including `money`) then item rows with parameterized
     `INSERT` statements (no `OR REPLACE` / upsert) using durable columns only:
     - passwords: `character_id`, `login`, `password`, `money`
     - items: `id`, `character_id`, `login`, `cell`, `vnum`, `count`, `locked`
       (`locked` bool → SQL integer `0`/`1`);
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name` (`15` / `character_safebox_money`);
   - `character_count` / `password_count` / `item_count`;
   - sorted `character_ids`.
   It never includes password/item payloads, SQL text, DSNs, or safebox
   snapshot bytes.
4. Empty quarantined exports (present empty `passwords` / `items` slices)
   succeed as a no-op transaction after the ledger gate (counts `0`).
5. Duplicate primary keys / unique-index collisions fail closed and roll the
   transaction back (no partial import).
6. Missing parent `characters` rows fail closed via FK (or equivalent engine
   error) and roll the transaction back — callers must import `0002` roster
   first (or otherwise ensure parents exist).
7. The primitive does **not**:
   - mutate bootstrap FileStores / MemoryStores;
   - change `schema_migrations` rows;
   - open a DSN itself or register a production driver;
   - expose a `gamed` / `authd` ops mutation route;
   - import `0010` ground / other tip domains;
   - invent upsert / merge / truncate-and-reload policy.
8. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0015` on a temp SQLite DB, imports a quarantined sample roster then
   safebox-state export, and `SELECT`s the durable rows back (including zero
   money, empty password, and locked mapping). Default untagged `go test ./...`
   stays free of the SQLite dependency.
9. Docs mark Track E / migration-contract SQL-import follow-ups as owned for
   `0002` + `0003` + `0004` + `0011` + `0014`/`0015`; ground import, CLI wiring, production-engine selection, and automatic / scheduled
   purge execution remain deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).

## What this is not yet

- `metin2-migrate` CLI import command
- SQL import for `0010` ground (or other tip kinds)
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for login/select/safebox
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/safeboxstore/safebox_state_import.go` (new)
- `internal/safeboxstore/safebox_state_import_test.go` (new; untagged fail-closed cases)
- `internal/safeboxstore/safebox_state_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic / no BeginTx
- sqlite harness: apply to `>= 15` → import roster parents → import sample
  safebox state → SELECT matches passwords (incl. money) / items / locked
- sqlite harness: second import of the same keys fails closed (unique conflict)
- sqlite harness: import before migrations / without `0015` fails closed
- sqlite harness: import without parent character rows fails closed (FK)
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/safeboxstore -run 'CharacterSafeboxStateImport' -count=1
go test -tags=sqlite_harness ./internal/safeboxstore -run 'SQLiteHarness.*SafeboxStateImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic `0015` import primitive is green under the SQLite harness
- Track E docs name `0002`+`0003`+`0004`+`0011`+`0014`/`0015` SQL import as
  owned and keep ground / CLI / engine selection deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
