# Character quest-state SQL import/backfill — 2026-08-26

## Objective

Add the third programmatic SQL import/backfill seam for quarantined
`0004_character_quest_state` exports: validate through the existing quarantine
contract, insert rows into `character_quest_flags` inside one transaction, and
prove the round-trip on the build-tagged SQLite harness after parent `0002`
roster rows exist.

This extends Track E SQL import beyond the landed `0002` roster and `0003`
item-state seams — without selecting a production engine, shipping a stock
release driver, wiring a daemon/ops mutation endpoint, inventing upsert/
idempotent rewrite policy, or importing `0011+` / other tip domains.

## Why now

- `accountstore.ImportAccountCharacterRoster` already owns the FK parents
  (`accounts` / `characters`) under the SQLite harness.
- Export + quarantine for `0004` quest-state are green offline and on loopback;
  the missing cohesive step is INSERT execution against a real engine for the
  first child schema that the PvE kill-quest / service-gate vertical already
  rematerializes from the bootstrap FileStore.
- Track E and migration-contract follow-ups still name `0004+` SQL import as
  deferred after `0003`. Owning quest-state next keeps the playable quest-flag
  path honest before point / ground / safebox child imports.

## Contract frozen by this slice

1. New primitive lives in `internal/queststate`:
   - `ImportCharacterQuestState(ctx, executor, export) (CharacterQuestStateImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineCharacterQuestStateExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger entry for version `4` /
     `character_quest_state` (empty/missing ledger or tip `< 4` fail closed);
   - inserts flag rows with parameterized `INSERT` statements (no `OR REPLACE`
     / upsert) using durable columns only:
     `character_id`, `quest_ref`, `flag_name`, `value`
     (the export's `character` name remains operator aid and is not written);
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name` (`4` / `character_quest_state`);
   - `character_count` / `flag_count`;
   - sorted `character_ids`.
   It never includes flag payloads, SQL text, DSNs, or quest-state snapshot
   bytes.
4. Empty quarantined exports (present empty `flags` slice) succeed as a no-op
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
   - import `0011+` / ground / safebox / other tip domains;
   - invent upsert / merge / truncate-and-reload policy.
8. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0004` on a temp SQLite DB, imports a quarantined sample roster then
   quest-state export, and `SELECT`s the durable rows back. Default untagged
   `go test ./...` stays free of the SQLite dependency.
9. Docs mark Track E / migration-contract SQL-import follow-ups as owned for
   `0002` + `0003` + `0004`; `0011+` / ground / safebox imports, CLI wiring, production-engine selection, and automatic / scheduled
   purge execution remain deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).

## What this is not yet

- `metin2-migrate` CLI import command
- SQL import for `0011` points / `0010` ground / `0014` / `0015` safebox (or
  other tip kinds)
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for login/select/quest flags
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/queststate/quest_state_import.go` (new)
- `internal/queststate/quest_state_import_test.go` (new; untagged fail-closed cases)
- `internal/queststate/quest_state_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic / no BeginTx
- sqlite harness: apply to `>= 4` → import roster parents → import sample quest
  state → SELECT matches character_id / quest_ref / flag_name / value
- sqlite harness: second import of the same keys fails closed (unique conflict)
- sqlite harness: import before migrations / without `0004` fails closed
- sqlite harness: import without parent character rows fails closed (FK)
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/queststate -run 'CharacterQuestStateImport' -count=1
go test -tags=sqlite_harness ./internal/queststate -run 'SQLiteHarness.*QuestStateImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic `0004` import primitive is green under the SQLite harness
- Track E docs name `0002`+`0003`+`0004` SQL import as owned and keep `0011+` /
  CLI / engine selection deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
