# Character point-state SQL import/backfill — 2026-08-26

## Objective

Add the fourth programmatic SQL import/backfill seam for quarantined
`0011_character_point_state` exports: validate through the existing quarantine
contract, insert rows into `character_points` inside one transaction, and prove
the round-trip on the build-tagged SQLite harness after parent `0002` roster
rows exist.

This extends Track E SQL import beyond the landed `0002` roster, `0003`
item-state, and `0004` quest-state seams — without selecting a production
engine, shipping a stock release driver, wiring a daemon/ops mutation endpoint,
inventing upsert/idempotent rewrite policy, or importing `0010` ground /
`0014`/`0015` safebox / other tip domains.

## Why now

- `accountstore.ImportAccountCharacterRoster` already owns the FK parents
  (`accounts` / `characters`) under the SQLite harness.
- Export + quarantine for `0011` point-state are green offline and on loopback;
  the missing cohesive step is INSERT execution against a real engine for the
  fixed-width selected-character point vector that the PvE death-floor /
  restart / item-use rematerialize path already persists in bootstrap snapshots.
- Track E and migration-contract follow-ups still name `0011+` SQL import as
  deferred after `0004`. Owning point-state next keeps HP/gold-adjacent point
  durability honest before ground / safebox child imports.

## Contract frozen by this slice

1. New primitive lives in `internal/accountstore`:
   - `ImportCharacterPointState(ctx, executor, export) (CharacterPointStateImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineCharacterPointStateExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger entry for version `11` /
     `character_point_state` (empty/missing ledger or tip `< 11` fail closed);
   - inserts point rows with parameterized `INSERT` statements (no `OR REPLACE`
     / upsert) using durable columns only:
     `character_id`, `point_index`, `value`
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name` (`11` / `character_point_state`);
   - `character_count` / `point_row_count`;
   - sorted `character_ids`.
   It never includes point payloads, SQL text, DSNs, or account snapshot bytes.
4. Empty quarantined exports (present empty `points` slice) succeed as a no-op
   transaction after the ledger gate (counts `0`).
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
   - import `0010` ground / `0014` / `0015` safebox / other tip domains;
   - invent upsert / merge / truncate-and-reload policy.
8. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0011` on a temp SQLite DB, imports a quarantined sample roster then
   point-state export, and `SELECT`s the durable rows back (including zero and
   negative signed values). Default untagged `go test ./...` stays free of the
   SQLite dependency.
9. Docs mark Track E / migration-contract SQL-import follow-ups as owned for
   `0002` + `0003` + `0004` + `0011`; ground / safebox imports, CLI wiring, production-engine selection, and automatic / scheduled
   purge execution remain deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).

## What this is not yet

- `metin2-migrate` CLI import command
- SQL import for `0010` ground / `0014` / `0015` safebox (or other tip kinds)
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for login/select/points
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/accountstore/point_state_import.go` (new)
- `internal/accountstore/point_state_import_test.go` (new; untagged fail-closed cases)
- `internal/accountstore/point_state_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic / no BeginTx
- sqlite harness: apply to `>= 11` → import roster parents → import sample point
  state → SELECT matches character_id / point_index / value (incl. negatives/zeros)
- sqlite harness: second import of the same keys fails closed (unique conflict)
- sqlite harness: import before migrations / without `0011` fails closed
- sqlite harness: import without parent character rows fails closed (FK)
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/accountstore -run 'CharacterPointStateImport' -count=1
go test -tags=sqlite_harness ./internal/accountstore -run 'SQLiteHarness.*PointStateImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic `0011` import primitive is green under the SQLite harness
- Track E docs name `0002`+`0003`+`0004`+`0011` SQL import as owned and keep
  ground / safebox / CLI / engine selection deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
