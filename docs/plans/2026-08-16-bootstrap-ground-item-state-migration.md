# Bootstrap Ground-Item State Migration — 2026-08-16

## Objective

Add the first project-owned SQL migration contract for pending bootstrap ground handles without pretending the shipped runtime is DB-backed.

The game runtime already owns temporary ground item and ground gold handles for accepted drops, fixed reward drops, visible pickup, transfer/visibility rebuilds, and loopback runtime snapshots. Before adding any durable repository or restart recovery for those handles, the migration catalog now freezes the schema boundary that future backfill/import work must target.

## Contract frozen by this slice

`0010_bootstrap_ground_item_state` adds one schema-only table:

- `bootstrap_ground_items`

The table captures the current runtime/debug snapshot identity for pending bootstrap ground entries:

- `vid` — the current client-visible bootstrap ground handle, primary key, non-zero `uint32` range;
- `vnum` — item template / gold marker vnum, non-zero `uint32` range;
- `item_count` — non-null only for item-shaped ground entries, `1..255` to match the current visible pickup carrier;
- `gold_amount` — non-null only for gold-shaped entries, `1..2147483647`, with `vnum = 1` for the current bootstrap currency marker;
- owner identity: `owner_login`, `owner_character_id`, `owner_vid`, and fixed-width-safe `owner_name`;
- location: `map_index`, `x`, `y`, `z`;
- `pickup_range`, defaulting to the current bootstrap reach of `300`;
- `created_at` / `updated_at` timestamps.

The migration also adds:

- a foreign-key boundary from `owner_character_id` to `characters(id)`, tying persisted bootstrap handles back to the already-owned roster migration;
- a map/location index for future map-scoped recovery and visibility preflights;
- an owner identity index for future owner cleanup and stale-owner reconciliation.

The down migration drops only `bootstrap_ground_items`.

## What this is not yet

This slice deliberately does not add:

- a DB-backed ground-item repository;
- process-restart restoration of pending ground handles;
- ownership timer persistence;
- public ownership release policy;
- party loot ownership tables;
- item sockets/bonuses for ground entries;
- a JSON export/import/backfill tool for live ground handles;
- any daemon-local mutating migration endpoint.

The shipped runtime still keeps pending bootstrap ground handles in memory. `0010` is a durable schema contract and planning/checksum boundary only.

## Why this order

Ground entries have grown beyond a single self-visible drop echo: they now participate in pickup, owner delivery, gold pickup, transfer rebootstrap, map occupancy, and runtime introspection. Freezing the SQL shape first gives the persistence lane a safe next boundary before any crash/restart recovery or DB-backed repository work tries to serialize live world state.

The schema is intentionally narrow and mirrors only already-owned bootstrap fields. It does not try to design final Metin2 loot policy.

## TDD and validation

Focused coverage in `db/migrations` proves:

- the embedded catalog now includes `0010_bootstrap_ground_item_state` after `0009_item_template_refine_info`;
- both SQL files are manifest-pinned by SHA-256;
- the up migration owns the table, owner/location/gold/item constraints, foreign-key boundary, and indexes;
- the down migration drops the table;
- dry-run planning includes the new tenth pending step from an empty ledger.

Adjacent runtime/ops/CLI tests were updated where they assert catalog tip metadata.

Validation for this slice:

- `go test ./db/migrations`
- `go test ./internal/minimal -run 'TestGameRuntimeMigrationStatusPlansBuiltInCatalogWithoutExecutingSQL|TestGameRuntimeMigrationCatalogSummaryReturnsMetadataOnlyCatalog' -count=1`
- `go test ./internal/migratecli -run TestRunCatalogWritesMetadataOnlySummary -count=1`
- `go test ./internal/ops -run TestLocalMigrationStatusEndpointReturnsDryRunPlanForLoopbackGet -count=1`
- broader repo validation is recorded in the run summary.

## Follow-up options

1. Add a read-only migration-shaped export for currently pending runtime ground handles only if there is a stable operator need for live-state capture.
2. Add crash/restart recovery for pending ground entries only after deciding whether in-memory bootstrap handles should survive process restart at all.
3. Add ownership timer/public-release columns in a separate migration once timer semantics are owned.
4. Keep DB apply/rollback surfaces CLI-only and daemon ops endpoints read-only unless a future production-admin design explicitly changes that boundary.
