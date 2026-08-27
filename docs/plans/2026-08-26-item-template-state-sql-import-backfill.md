# Item-template-state SQL import/backfill — 2026-08-26

## Objective

Add the seventh programmatic SQL import/backfill seam for quarantined
`0009_item_template_refine_info` exports: validate through the existing
quarantine contract, insert rows into `item_templates` plus child socket /
attribute / use-effect / equip-effect / refine-info / refine-material tables
inside one transaction, and prove the round-trip on the build-tagged SQLite
harness after the catalog tip includes version `9`.

This extends Track E SQL import beyond the landed `0002` roster, `0003`
item-state, `0004` quest-state, `0010` ground-item-state, `0011` point-state,
and `0014`/`0015` safebox seams — without selecting a production engine,
shipping a stock release driver, wiring a daemon/ops mutation endpoint,
inventing upsert/idempotent rewrite policy, or claiming DB-backed live
item-template loading.

## Why now

- Export + quarantine for tip-`0009` item-template-state are green offline and
  on loopback; the missing cohesive step is INSERT execution against a real
  engine for the authored template surface the PvE vertical already mutates
  (merchant buy/sell, use/equip, refine preview, safebox anti-flag text).
- Track E and migration-contract follow-ups still name content-shaped SQL
  import (`0005`/`0006`/`0009`, then `0007`/`0008`/`0012`/`0013`) as deferred
  after the character/ground/safebox child imports. Owning templates next
  closes the first non-character content import beside the landed FileStore
  restart path.

## Contract frozen by this slice

1. New primitive lives in `internal/itemstore`:
   - `ImportItemTemplateState(ctx, executor, export) (ItemTemplateStateImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineItemTemplateStateExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger entry for version `9` /
     `item_template_refine_info` (empty/missing ledger or tip `< 9` fail closed);
   - inserts templates, then sockets, attributes, use effects, equip effects,
     refine infos, then refine materials with parameterized `INSERT` statements
     (no `OR REPLACE` / upsert) using durable columns only:
     - omit `created_at` / `updated_at` (database defaults);
     - map bool fields → SQL integer `0`/`1`;
     - bind `unique_item` from the export `Unique` / JSON `unique_item` column;
     - bind `safebox_reject_message` beside `anti_safebox`;
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name` (`9` / `item_template_refine_info`);
   - `template_count` / `socket_count` / `attribute_count` /
     `use_effect_count` / `equip_effect_count` / `refine_info_count` /
     `refine_material_count`;
   - sorted `vnums`.
   It never includes template payloads, SQL text, DSNs, or FileStore snapshot
   bytes.
4. Empty quarantined exports (present empty slices) succeed as a no-op
   transaction after the ledger gate (counts `0`).
5. Duplicate primary keys / unique-index collisions fail closed and roll the
   transaction back (no partial import).
6. The primitive does **not**:
   - mutate bootstrap FileStores / MemoryStores / live template indexes;
   - change `schema_migrations` rows;
   - open a DSN itself or register a production driver;
   - expose a `gamed` / `authd` ops mutation route;
   - invent upsert / merge / truncate-and-reload policy;
   - claim DB-backed runtime template loading (FileStore remains the restart
     path).
7. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0009` on a temp SQLite DB, imports a quarantined sample export
   (templates + sockets/attributes/effects/refine rows including
   `safebox_reject_message`), and `SELECT`s the durable rows back. Default
   untagged `go test ./...` stays free of the SQLite dependency.
8. Docs mark Track E / migration-contract SQL-import follow-ups as owned for
   `0002` + `0003` + `0004` + `0009` + `0010` + `0011` + `0014`/`0015`;
   `0007`/`0008`/`0012`/`0013` imports, CLI wiring, production-engine selection, upsert policy, and automatic / scheduled
   purge execution remain deferred. ~~Folding purge into `contrib/lab-retention-gc`~~ Done — see [contrib artifact GC-aside purge print helper](2026-08-27-contrib-artifact-gc-aside-purge-print-helper.md).

## What this is not yet

- `metin2-migrate` CLI import command
- SQL import for `0007` login-ticket / `0008`+`0012`+`0013` static-actor content
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for item templates
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/itemstore/item_template_state_import.go` (new)
- `internal/itemstore/item_template_state_import_test.go` (new; untagged fail-closed cases)
- `internal/itemstore/item_template_state_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic / no BeginTx
- sqlite harness: apply to `>= 9` → import sample template state → SELECT matches
  templates / children / bool→int / safebox_reject_message / refine rows
- sqlite harness: second import of the same vnums fails closed (unique conflict)
- sqlite harness: import before migrations / without `0009` fails closed
- sqlite harness: empty export succeeds as no-op after ledger gate
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/itemstore -run 'ItemTemplateStateImport' -count=1
go test -tags=sqlite_harness ./internal/itemstore -run 'SQLiteHarness.*ItemTemplateStateImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic `0009` import primitive is green under the SQLite harness
- Track E docs name `0002`+`0003`+`0004`+`0009`+`0010`+`0011`+`0014`/`0015`
  SQL import as owned and keep `0007`/`0008`/`0012`/`0013` / CLI / engine
  selection deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
