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
- the embedded catalog now also includes `0003_character_item_state`, a schema-only contract for selected-character item surfaces that are already persisted in the bootstrap account snapshot:
  - `character_inventory_items` stores carried item instance id, owning character id, carried slot, vnum, count, lock flag, and timestamps,
  - `character_equipment_items` stores equipped item instance id, owning character id, named equipment slot, vnum, count, lock flag, and timestamps,
  - `character_quickslots` stores owning character id plus quickslot position/type/slot tuples,
  - item ids are globally unique across carried and equipped item rows by table primary keys and exporter validation, while `(character_id, slot)`, `(character_id, equip_slot)`, and `(character_id, position)` are unique within their surfaces,
  - item templates, login tickets, authored content, world runtime state, exchange/trade state, ground-item handles, and migration execution remain out of scope.
- the embedded catalog now also includes `0004_character_quest_state`, a schema-only contract for the standalone bootstrap quest-state snapshot:
  - `character_quest_flags` stores owning character id, quest ref, flag name, non-zero value, and timestamps,
  - `(character_id, quest_ref, flag_name)` is the primary key so duplicate flags fail closed at the database boundary,
  - `quest_ref` has a read index for future quest-focused inspections or backfill checks,
  - client-visible quest packets, quest scripts, item templates, login tickets, authored content, world runtime state, and migration execution remain out of scope.
- the embedded catalog now also includes `0005_item_template_state`, a schema-only contract for authored item-template snapshots that are already loaded from bootstrap JSON:
  - `item_templates` stores one row per authored `vnum` with owned stack, pricing, client flag, anti-flag, appearance, equipment-slot, rejection-text, and pickup-range metadata,
  - `item_template_sockets`, `item_template_attributes`, `item_template_use_effects`, and `item_template_equip_effects` split fixed display arrays and optional point-effect payloads into child tables keyed by `vnum`,
  - validation preserves current bootstrap bounds such as `max_count <= 255`, buy price within `uint32`, sell price within the signed point-change carrier, owned equipment-slot names only, socket positions `0..2`, attribute positions `0..6`, and effect point-index bounds,
  - runtime item-template loading still comes from the file-backed store or built-in fallback; migration execution, item-template DB repositories, content-bundle DB storage, and runtime DB writes remain out of scope.
- the embedded catalog now also includes `0006_item_template_safebox_reject_message`, an additive schema-only contract for authored safebox rejection text on item templates:
  - it adds `safebox_reject_message` to `item_templates`,
  - the column is constrained so non-empty text requires `anti_safebox = 1`, matching the current fail-closed storage feedback rule,
  - runtime item-template loading remains file-backed.
- the embedded catalog now also includes `0007_auth_login_ticket_handoff`, a schema-only contract for the authd-to-gamed login-ticket handoff:
  - `auth_login_tickets` stores the non-zero `login_key`, non-empty `issued_at`, login/original-normalized login, empire, nullable `consumed_at`, and a transitional `characters_snapshot_json` payload,
  - active tickets have a partial unique index on `login_key` where `consumed_at IS NULL`, so future SQL-backed stores can preserve the one-shot active-key boundary without overwriting live handoffs,
  - the migration deliberately does not change the current JSON ticket issue/load/consume runtime and does not add a DB repository or migration executor.
- the embedded catalog now also includes `0008_static_actor_content_state`, a schema-only contract for authored static actors and interaction definitions:
  - `interaction_definitions` stores the current authored `info`, `talk`, `warp`, and `shop_preview` definition payloads keyed by `(kind, ref)`,
  - `interaction_merchant_catalog_entries` stores structured merchant-preview catalog rows with the current slot, vnum, price, and count bounds,
  - `static_actors` stores authored visible/service/spawn-backed actor placement, race, optional interaction ref, optional spawn-group ref, optional spawn home, optional combat profile, and reward experience/gold,
  - `static_actor_reward_drops` stores ordered reward drop vnums for spawn-backed actors,
  - the migration deliberately does not make content DB-backed at runtime and does not yet add migration-shaped exports/imports for the static actor or interaction JSON stores.
- the embedded catalog now also includes `0009_item_template_refine_info`, an additive schema-only contract for template-authored refine-dialog preview metadata:
  - `item_template_refine_infos` stores the preview result vnum, cost, and probability for templates that are already validated as `refineable`,
  - `item_template_refine_materials` stores up to five ordered material rows for each preview,
  - the migration deliberately does not add accepted refine result semantics, inventory mutation, success/failure policy, or a DB-backed item-template runtime repository.
- `BuiltInCatalogSummary` and `gamed`'s loopback-only read-only `GET /local/db/migrations/catalog` endpoint expose a metadata-only inventory of the validated embedded migration catalog (`latest_version`, paths, and up/down checksums) without opening a configured DB or exposing executable SQL.
- `gamed` exposes a loopback-only read-only `GET /local/db/migrations/status` endpoint that returns the same metadata-only dry-run plan; with no DB config it plans against an empty ledger, and with explicit DB config it reads only the `schema_migrations` ledger before planning.
- `ReadSQLLedgerEntries` / `PlanUpToLatestFromSQLLedger` add the first database/sql-compatible ledger reader seam for future preflight tooling: callers supply a `QueryContext` boundary, the package reads only `version`, `name`, and `up_sha256` from `schema_migrations` in version order, closes rows, and fails closed on query, scan, iteration, close, catalog, or ledger drift errors.
- `PlanToVersion` / `PlanCatalogToVersion` and the matching SQL-ledger variants now provide an explicit target-version dry-run contract:
  - target `latest` is exposed by the existing latest-status path,
  - target `0` previews a complete rollback using down migrations in reverse applied-version order,
  - intermediate targets emit only the up/down steps needed to move from the current ledger version to that target,
  - out-of-range targets fail closed with `ErrInvalidMigrationTarget`,
  - target plans still expose metadata only and never include executable SQL.
- `LedgerSnapshot` / `ReadJSONLedgerSnapshot` / `MarshalJSONLedgerSnapshot` freeze an offline JSON ledger-snapshot contract for operators and future CLI tooling that cannot or should not open the live DB directly:
  - the format marker is `go-metin2-schema-migrations-ledger-v1`,
  - entries contain only `version`, `name`, and `up_sha256`, mirroring the durable `schema_migrations` ledger without SQL text,
  - decoding is strict and fails closed for invalid UTF-8, malformed JSON, unknown fields, trailing JSON, missing/null entries, invalid names, invalid checksums, zero/negative versions, or duplicate versions,
  - marshaling sorts entries by version without mutating the input and emits an explicit empty `entries: []` array for an empty applied ledger,
  - `PlanToVersionFromLedgerSnapshot` and JSON-reader variants reuse the same catalog/ledger validation and target-version planner used by direct and SQL-ledger paths.
- `LedgerSnapshotFromSQLLedger` and `gamed`'s loopback-only `GET /local/db/migrations/ledger-snapshot` endpoint can now export the same strict offline snapshot shape from the configured DB-preflight ledger target:
  - when DB preflight is disabled, the runtime returns an explicit empty snapshot instead of opening a DB,
  - when driver/DSN are configured, the helper reads only `version`, `name`, and `up_sha256` from `schema_migrations`, validates/sorts the rows through the same snapshot boundary, closes the DB connection, and exposes no executable SQL or DSN,
  - wrong method, non-loopback callers, unavailable drivers, ledger query/scan failures, and malformed ledger rows fail closed.
- `gamed` exposes the offline ledger-snapshot planner through loopback-only `POST /local/db/migrations/plan-from-ledger-snapshot?target_version=N`:
  - the request body is the strict metadata-only ledger snapshot and is bounded to 64 KiB,
  - invalid target queries or invalid snapshots fail with `400`, oversized bodies fail with `413`, planner/catalog/ledger drift fails with `409`, and non-loopback callers fail with `403`,
  - the response is the same metadata-only `Plan` shape and the endpoint does not open a configured DB, execute SQL, apply migrations, roll back migrations, or mutate `schema_migrations`.
- runtime configuration now carries an optional DB preflight boundary:
  - `METIN2_DB_DRIVER` / `METIN2_GAMED_DB_DRIVER`,
  - `METIN2_DB_DSN` / `METIN2_GAMED_DB_DSN`,
  - both empty means DB-backed migration preflight is disabled,
  - partial, malformed, or unavailable configured drivers fail startup validation,
  - configured status reads through `database/sql` but does not bundle or select a real driver dependency yet, so stock builds without a linked driver must keep DB preflight disabled,
  - `/local/runtime-config` reports only `database.configured`, `database.driver`, and `database.dsn_configured`; it never exposes the DSN value.
- `ApplyUpToLatest` / `ApplyToVersion` / `ApplyCatalogUpToVersion` add the first programmatic migration apply primitive:
  - callers still supply the current applied ledger and a `database/sql` transaction boundary instead of letting the migration package own driver selection, DSNs, or connection pools,
  - the same catalog/ledger/target validation used by dry-run planning runs before any transaction is opened,
  - pending up migration SQL is split on conservative statement boundaries and each terminated statement executes before its matching `schema_migrations` ledger insert,
  - pending down migrations delete the matching `schema_migrations` row by `version`, `name`, and `up_sha256` before executing each terminated down statement, preserving rollback-to-zero ordering for the `schema_migrations` table,
  - ledger insert/delete row-count drift fails closed when the database reports anything other than exactly one affected row,
  - migration SQL failures, ledger insert/delete failures, or commit failures return errors; migration/ledger failures attempt to roll back and report rollback errors with the original failure,
  - the returned `ApplyResult` exposes only version metadata and applied plan steps, not executable SQL text, DSNs, or row data,
  - this primitive is not exposed through `gamed`'s local ops mux and is not wired into daemon startup.
- `internal/accountstore` now exposes a read-only account/character roster projection for the `0002_account_character_roster` migration boundary:
  - export rows carry the migration version/name so operators know which schema contract they target,
  - account rows are deterministic by normalized login and include stable project-owned ids, original login, normalized login, and empire,
  - character rows include only non-empty select-screen slots in account/slot order and mirror the roster migration fields,
  - inventory, equipment, quickslots, quest state, login tickets, authored content, and world runtime state are deliberately omitted,
  - invalid snapshots that would violate the schema shape fail closed instead of being silently coerced.
- `gamed` exposes the projection through loopback-only read-only `GET /local/account-store/exports/account-character-roster`; it reads the committed bootstrap account snapshots, returns JSON, and performs no SQL or store mutation.
- `internal/accountstore` now exposes a second read-only character item-state projection for the `0003_character_item_state` migration boundary:
  - inventory rows are deterministic by account/character/slot and include item id, character id, slot, vnum, count, and lock flag,
  - equipment rows are deterministic by account/character/equipment-slot order and include item id, character id, named equipment slot, vnum, count, and lock flag,
  - quickslot rows are deterministic by account/character/position and include character id, position, type, and slot,
  - malformed items, duplicate item ids across carried/equipped state, invalid quickslots, or invalid roster prerequisites fail closed.
- `gamed` exposes this projection through loopback-only read-only `GET /local/account-store/exports/character-item-state`; it reads committed bootstrap account snapshots, returns JSON, and performs no SQL or store mutation.
- `internal/queststate` now exposes a read-only character quest-state projection for the `0004_character_quest_state` migration boundary:
  - quest flag rows are deterministic by normalized snapshot order and include resolved character id, character name, quest ref, flag name, and value,
  - the projection resolves quest-state character names through the committed account/character roster export, so flags for missing characters fail closed instead of becoming orphan rows,
  - missing quest-state snapshots produce an empty migration-shaped export, matching the existing validation semantics for an absent standalone quest-state file.
- `gamed` exposes this projection through loopback-only read-only `GET /local/quest-state/exports/character-quest-state`; it reads committed bootstrap account and quest-state snapshots, returns JSON, and performs no SQL or store mutation.
- `internal/itemstore` now exposes a read-only item-template-state projection for the current item-template migration boundary (`0009_item_template_refine_info`, after the base `0005_item_template_state` schema and the additive `0006_item_template_safebox_reject_message` storage guard):
  - template rows are deterministic by `vnum` and include the owned item-template metadata already validated by the file-backed store,
  - socket and attribute rows include only non-zero authored display entries with fixed positions,
  - use-effect and equip-effect rows include the optional point-effect payloads when present, with default use-effect `consume_count` resolved to `1`,
  - refine-info and refine-material rows include optional template-authored refine-dialog preview metadata with deterministic material positions,
  - missing item-template snapshots produce an empty migration-shaped export rather than forcing built-in bootstrap fallback rows into a future DB import.
- `gamed` exposes this projection through loopback-only read-only `GET /local/item-templates/exports/item-template-state`; it reads the committed authored item-template snapshot, returns JSON, and performs no SQL or store mutation.
- `internal/loginticket` now exposes a read-only auth login-ticket handoff projection for the `0007_auth_login_ticket_handoff` migration boundary:
  - ticket rows are deterministic by normalized login, original login, and login key,
  - rows include active non-zero login key, UTC issued timestamp, login/original-normalized identity, empire, nil consumed timestamp for the current pending-ticket store, and a transitional character snapshot JSON payload,
  - the projection validates pending committed tickets through the same file-store boundary and does not consume tickets, emit SQL, apply migrations, or mutate the login-ticket store.
- `gamed` exposes this projection through loopback-only read-only `GET /local/login-tickets/exports/auth-login-ticket-handoff`; it reads committed pending bootstrap login tickets, returns JSON, and performs no SQL or store mutation.

The first migration is `0001_bootstrap_schema_migrations` and creates only a minimal `schema_migrations` ledger:

- `version INTEGER PRIMARY KEY`,
- `name TEXT NOT NULL`,
- `up_sha256 TEXT NOT NULL`,
- `applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP`.

The `up_sha256` column intentionally pins the exact SQL body that was applied, so a future migrator can refuse to treat a mutated historical migration as already applied.

The second migration is `0002_account_character_roster`. It is the first domain schema contract. The shipped daemons still load and save accounts/characters through the bootstrap file store, but the file store can now produce a deterministic schema-shaped roster export for operator inspection and future backfill tooling.

The third migration is `0003_character_item_state`. It is the first schema contract for item-bearing selected-character state already present in durable account snapshots: carried inventory, equipped items, and quickslots. Like the roster migration, this is still a projection/backfill boundary rather than a runtime DB implementation.

The fourth migration is `0004_character_quest_state`. It is the first schema contract for the standalone quest-state primitive that already exists as bootstrap JSON. The export resolves flags onto character ids from the roster projection before emitting rows, so malformed or orphaned quest flags are rejected during operator/backfill preflight rather than being silently imported later.

The fifth migration is `0005_item_template_state`. It freezes the first schema contract for authored bootstrap item templates, including the main template metadata row plus display socket/attribute child rows and optional use/equip effect child rows. The export deliberately reads only the committed authored item-template snapshot: missing snapshots produce an empty export instead of importing built-in fallback templates as if they were durable authored data.

The sixth migration is `0006_item_template_safebox_reject_message`. It adds the current authored safebox rejection text column to the item-template schema boundary while preserving the file-backed runtime and export-only posture.

The seventh migration is `0007_auth_login_ticket_handoff`. It freezes the first schema contract for the authd-to-gamed login-key handoff: active non-zero login keys, issued timestamps, login identity, empire context, optional consumed timestamp, and a transitional character snapshot JSON payload for the current select-surface handoff. It is still schema-only; the shipped runtime continues to use the JSON `internal/loginticket` store.

The eighth migration is `0008_static_actor_content_state`. It freezes the first schema contract for authored static actors and interaction definitions: current `info` / `talk` / `warp` / `shop_preview` definitions, merchant-preview catalog entries, visible/static actor placement and refs, spawn-backed actor refs, spawn homes, combat-profile names, and ordered reward drops. It is still schema-only; the shipped runtime continues to use the JSON `internal/staticstore` and `internal/interactionstore` stores, and no static actor or interaction export/import path is added yet.

The ninth migration is `0009_item_template_refine_info`. It extends the authored item-template schema boundary with optional refine-dialog preview rows (`item_template_refine_infos` and `item_template_refine_materials`) that mirror the already-owned file-backed `refine_info` metadata. It is still schema/export-only; accepted refine result semantics, item/economy mutation, and DB-backed template loading remain future work.

## What this is not yet

This is not a database runtime implementation. It deliberately does not add:

- a DB driver dependency or default production DB engine,
- DB connection pool ownership beyond the read-only migration-status preflight,
- production migration CLI/ops apply or rollback commands,
- account/character/item repository implementations or DB-backed runtime writes,
- JSON snapshot import/backfill execution tooling,
- accepted refine result execution,
- production deployment scripts.

The dry-run planner added on top of the catalog remains the only daemon-exposed migration behavior: callers can supply already-read ledger rows directly, provide a `database/sql`-compatible query boundary for the same metadata through `ReadSQLLedgerEntries` / `PlanUpToLatestFromSQLLedger` / `PlanToVersionFromSQLLedger`, export that configured metadata into a strict offline `LedgerSnapshot` through `LedgerSnapshotFromSQLLedger`, or provide a strict offline JSON `LedgerSnapshot` through `ReadJSONLedgerSnapshot` / `PlanToVersionFromLedgerSnapshot` when planning from copied ledger metadata. The first loopback ops endpoints use an empty ledger when DB config is disabled and a configured `database/sql` ledger reader when both driver and DSN are set. `/local/db/migrations/status` reports the latest-version target; `/local/db/migrations/plan?target_version=N` previews an explicit target such as rollback-to-zero; `/local/db/migrations/ledger-snapshot` exports only the current ledger metadata as strict offline JSON; `/local/db/migrations/plan-from-ledger-snapshot?target_version=N` accepts a bounded metadata-only snapshot body and produces the same plan shape without opening a configured DB. The account/character roster, character item-state, character quest-state, item-template-state, auth login-ticket handoff, and static actor content-state exports are also read-only: they map committed JSON snapshots to the existing schema shapes but do not insert rows, allocate a real production identity sequence, consume tickets, execute refine results, or quarantine/import data. The apply primitive executes pending up or down SQL only when a caller explicitly supplies a transaction-capable executor and applied ledger, and it is not reachable from the shipped daemons. The SQL ledger seam, offline ledger snapshot seam, runtime config, exports, and programmatic apply primitive are safe boundaries for later production migration tooling, not a substitute for that tooling.

Those require separate slices because each one changes operator and data-safety semantics.

## Likely next slices

1. Define a narrow account/character/item/quest-state/login-ticket/static-content repository interface backed by current tests before adding a DB implementation.
2. Add JSON-file-store import/quarantine tooling that consumes the exported `0002_account_character_roster`, `0003_character_item_state`, `0004_character_quest_state`, current item-template-state (`0009_item_template_refine_info`), `0007_auth_login_ticket_handoff`, and `0008_static_actor_content_state` shapes plus optional offline ledger snapshots without silently coercing bad snapshots.
3. Add a driver-backed test harness or build-tagged integration test for `schema_migrations` status and ledger-snapshot generation before adding apply/rollback tooling.
4. Add explicit migrations for richer item/economy domains, item ownership timers, combat-profile defaults, ground items, or world runtime state only after the account/character/item/item-template/login-ticket/static-content repository seams are stable.
5. Add a production apply/rollback command only after the dry-run status boundary, the programmatic apply primitive, and ledger validation behavior are exercised against an actual driver-backed test database, with explicit backup/restore policy.
6. Document production DB configuration, backups, and rollback policy once there is an actual DB-backed store.

## Exit criteria for this slice

- `go test ./db/migrations` validates the catalog, schema ledger migration, account/character roster migration, character item-state migration, character quest-state migration, item-template-state / safebox-reject / refine-info migrations, auth login-ticket handoff migration, static actor content-state migration, direct-ledger dry-run planning rules, explicit up/down target planning, the database/sql-compatible ledger-reader seam, configured-ledger snapshot export, and the strict offline JSON ledger-snapshot plan boundary.
- `go test ./internal/config ./internal/minimal ./internal/service ./internal/ops` validates optional DB config loading, startup fail-closed behavior for partial config, no-DSN runtime-config exposure, the configured-driver migration-status boundary, loopback-only explicit migration-plan previews including the ledger-snapshot GET export and POST preflight, and the local account/character roster, character item-state, auth login-ticket handoff, character quest-state, item-template-state, and static actor content-state export endpoints.
- `go test ./internal/accountstore ./internal/loginticket ./internal/queststate ./internal/itemstore ./internal/staticstore` validates deterministic `0002_account_character_roster`, `0003_character_item_state`, `0004_character_quest_state`, `0007_auth_login_ticket_handoff`, current `0009_item_template_refine_info` item-template-state, and `0008_static_actor_content_state` export rows plus fail-closed schema-shape checks for bootstrap account, login-ticket, quest-state, item-template, static-actor, and interaction snapshots.
- `go test ./...` and `go vet ./...` remain green.
- README/development docs describe `db/migrations` as the validated migration catalog and read-only planning skeleton, including the schema-only login-ticket handoff boundary, not a finished DB layer.
