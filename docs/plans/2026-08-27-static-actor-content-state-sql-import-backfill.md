# Static-actor content-state SQL import/backfill — 2026-08-27

## Objective

Add the ninth programmatic SQL import/backfill seam for quarantined
`0013_static_actor_combat_profile_state` exports: validate through the existing
quarantine contract, insert rows into the tip-`0013` static-actor / interaction /
combat-profile tables inside one transaction, and prove the round-trip on the
build-tagged SQLite harness after the catalog tip includes version `13`.

This extends Track E SQL import beyond the landed `0002` roster, `0003`
item-state, `0004` quest-state, `0007` login-ticket, `0009` item-template-state,
`0010` ground-item-state, `0011` point-state, and `0014`/`0015` safebox seams —
without selecting a production engine, shipping a stock release driver, wiring a
daemon/ops mutation endpoint, inventing upsert/idempotent rewrite policy, or
claiming DB-backed live static-actor / interaction loading.

## Why now

- Export + quarantine for tip-`0013` static-actor content-state are green offline
  and on loopback; the missing cohesive step is INSERT execution against a real
  engine for the authored NPC / spawn / combat-profile surface the PvE vertical
  already uses (map visibility, merchant preview, safebox NPC, quest_flag turn-in,
  practice-mob rewards, portable combat profiles).
- Track E and migration-contract follow-ups still name content-shaped SQL import
  (`0008`/`0012`/`0013`) as deferred after the character/ground/safebox/template/
  ticket imports. Owning tip-`0013` next closes the last quarantined catalog tip
  without a programmatic import seam.

## Contract frozen by this slice

1. New primitive lives in `internal/staticstore`:
   - `ImportStaticActorContentState(ctx, executor, export) (StaticActorContentStateImportResult, error)`
2. Before any SQL mutation the importer:
   - rejects a nil executor;
   - runs `QuarantineStaticActorContentStateExport` (fail closed on shape drift);
   - opens one transaction;
   - reads `schema_migrations` through `db/migrations.ReadSQLLedgerEntries` and
     requires an applied ledger entry for version `13` /
     `static_actor_combat_profile_state` (empty/missing ledger or tip `< 13`
     fail closed);
   - inserts rows with parameterized `INSERT` statements (no `OR REPLACE` /
     upsert) in FK-safe order using durable columns only:
     1. `interaction_definitions`
     2. `interaction_merchant_catalog_entries`
     3. `interaction_quest_flag_reward_items`
     4. `interaction_quest_flag_consume_items`
     5. `static_actors`
     6. `static_actor_reward_drops`
     7. `static_actor_combat_profiles`
     8. `static_actor_combat_profile_death_reward_drops`
   - omits `created_at` / `updated_at` (database defaults);
   - binds nullable tip-`0012`/`0013` columns as SQL NULL when absent:
     - definition `map_index` / `x` / `y`;
     - actor `spawn_home_*`, empty `interaction_kind` / `interaction_ref`, empty
       `spawn_group_ref`;
   - binds empty durable text/number defaults (`text`, `title`, `size`, quest /
     reward / consume fields, empty `combat_profile`) as present zero values;
   - commits only when every insert succeeds with the expected row counts.
3. Success result is metadata-only:
   - `migration_version` / `migration_name`
     (`13` / `static_actor_combat_profile_state`);
   - `interaction_definition_count` / `merchant_catalog_entry_count` /
     `quest_flag_reward_item_count` / `quest_flag_consume_item_count` /
     `static_actor_count` / `reward_drop_count` / `combat_profile_count` /
     `combat_profile_death_reward_drop_count`;
   - sorted `entity_ids`, sorted unique `interaction_kinds`, sorted
     `combat_profiles`.
   It never includes content payloads, SQL text, DSNs, or FileStore snapshot
   bytes.
4. Empty quarantined exports (present empty slices) succeed as a no-op
   transaction after the ledger gate (counts `0`).
5. Duplicate primary keys / unique-index collisions fail closed and roll the
   transaction back (no partial import).
6. The primitive does **not**:
   - mutate bootstrap FileStores / MemoryStores / live static-actor indexes;
   - change `schema_migrations` rows;
   - open a DSN itself or register a production driver;
   - expose a `gamed` / `authd` ops mutation route;
   - invent upsert / merge / truncate-and-reload policy;
   - claim DB-backed runtime content loading (JSON FileStores remain the restart
     path).
7. Build-tagged proof (`//go:build sqlite_harness`) applies the catalog at least
   through `0013` on a temp SQLite DB, imports a quarantined sample export
   covering tip-`0008`/`0012`/`0013` shapes (info/talk/warp/shop_preview,
   open_safebox, quest_flag reward/consume children, spawn-backed reward drops,
   kill-quest actor fields, and one portable combat profile with death-reward
   drops), and `SELECT`s the durable rows back. Default untagged `go test ./...`
   stays free of the SQLite dependency.
8. Docs mark Track E / migration-contract SQL-import follow-ups as owned for
   `0002` + `0003` + `0004` + `0007` + `0009` + `0010` + `0011` + `0013` tip
   (`0008`/`0012` historical content tips included via tip-`0013`) +
   `0014`/`0015`; CLI wiring, production-engine selection, upsert policy, and
   scheduled purge fold remain deferred.

## What this is not yet

- `metin2-migrate` CLI import command
- upsert / merge / truncate-and-reload policy
- production DB engine selection as a stock default
- DB-backed runtime repositories for static actors / interactions / combat profiles
- loopback ops mutation endpoint
- FreeBSD port / `pkg` enable defaults
- remote admin, secrets in git, metrics/tracing

## Likely files to change

- `internal/staticstore/static_actor_content_state_import.go` (new)
- `internal/staticstore/static_actor_content_state_import_test.go` (new; untagged fail-closed cases)
- `internal/staticstore/static_actor_content_state_import_sqlite_harness_test.go` (new; build-tagged)
- `docs/plans/2026-08-09-db-migration-contract.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` (Track E tip)
- `docs/development.md` (brief pointer)
- `docs/workflow/migration-apply-runbook.md` (one-line pointer)
- this plan

## TDD and validation

Focused coverage:

- nil executor / invalid export → error, no panic / no BeginTx
- sqlite harness: apply to `>= 13` → import sample tip-`0013` export → SELECT
  matches definitions / merchant / quest-flag children / actors / reward drops /
  combat profiles / death-reward drops
- sqlite harness: second import of the same primary keys fails closed (unique conflict)
- sqlite harness: import before migrations / without `0013` fails closed
- sqlite harness: empty export succeeds as no-op after ledger gate
- stdout/result never embeds DSN / executable SQL

Validation for this slice:

```bash
go test ./internal/staticstore -run 'StaticActorContentStateImport' -count=1
go test -tags=sqlite_harness ./internal/staticstore -run 'SQLiteHarness.*StaticActorContentStateImport' -count=1
gofmt -l on touched Go files
git diff --check
```

## Exit criteria

- programmatic tip-`0013` import primitive is green under the SQLite harness
- Track E docs name `0002`+`0003`+`0004`+`0007`+`0009`+`0010`+`0011`+`0013`+
  `0014`/`0015` SQL import as owned and keep CLI / engine selection / upsert
  deferred
- default untagged tests still do not require SQLite

## Anti-goals / ordering constraints

- Do not register SQLite in stock release binaries.
- Do not invent upsert/idempotent rewrite this slice.
- Do not fold purge into scheduled print helpers.
- Do not push `origin/main`; push only `origin/lane/persistence`.
