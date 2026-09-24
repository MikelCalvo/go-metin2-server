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

## Read-only live export follow-up

A later persistence-lane slice added a loopback-only read-only export for currently pending runtime ground handles:

- `GET /local/ground-items/exports/bootstrap-ground-item-state`

The export projects live item-shaped and gold-shaped ground snapshots onto the `0010_bootstrap_ground_item_state` row shape with `migration_version`, `migration_name`, and deterministic `ground_items` ordered by visible `vid`. It fails closed if any live handle cannot target the migration schema and includes the schema-owned pickup reach so operator/backfill tooling can inspect exactly the fields `0010` froze.

## Opt-in SQL repository

`worldruntime.SQLGroundItemStore` is the first live DB-backed pending ground repository beside this schema. A caller supplies the `database/sql` executor. `LoadGroundItems` / `SaveGroundItems` / `ExportBootstrapGroundItemState` require tip-`0010` plus additive `0026` sockets, `0029` attributes, and `0030` ownership timers. Save deletes every `bootstrap_ground_items` row and inserts the canonical export. Export identity stays tip-`0010`.

It does not select a driver, mount `gamed`, add a remote-admin or daemon mutation route, invent `ITEM_GROUND_ADD` wire sockets, or add party-loot tables. Process-local item ids stay on the ground-item FileStore, which remains stock process-restart rematerialize. Proven by `go test ./internal/worldruntime -run SQLGroundItemStore` and `go test -tags=sqlite_harness ./internal/worldruntime -run SQLiteHarnessSQLGroundItemStore`.

## What this is not yet

These slices deliberately do not add:

- stock `gamed` rematerialize from SQL (FileStore remains the restart path; opt-in `SQLGroundItemStore` is not mounted);
- party loot ownership tables;
- item sockets/attributes/ownership timers on the original `0010` table (additive `0026`/`0029`/`0030` later project those FileStore extras onto the same rows);
- a stock production driver or daemon-local mutating migration endpoint.

The shipped runtime still keeps pending bootstrap ground handles in memory and restarts them from the ground-item FileStore. `0010` remains the export identity. The opt-in SQL store is a caller-supplied Load/Save seam, not a cutover.

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

## Quarantine/preflight follow-up

A later persistence-lane slice added loopback-only quarantine for retained `0010` artifacts:

- `POST /local/ground-items/exports/bootstrap-ground-item-state/quarantine`

It validates and canonicalizes retained export JSON without opening a database, mutating live ground handles, or emitting SQL. See `docs/plans/2026-08-19-bootstrap-ground-item-state-quarantine.md`.

## CLI-only apply/rollback boundary

A later persistence-lane slice froze the operator-visible apply/rollback vs loopback-ops split without inventing production-admin daemon mutation or remote apply:

- `metin2-migrate apply-boundary` prints metadata-only `go-metin2-migration-apply-boundary-v1`
- the mutating schema surface stays CLI-only: `metin2-migrate apply` plus `--allow-rollback` for down plans
- loopback ops stay the closed read-only set: `GET /local/db/migrations/catalog`, `GET /local/db/migrations/status`, `GET /local/db/migrations/plan`, `GET /local/db/migrations/ledger-snapshot`, and `POST /local/db/migrations/plan-from-ledger-snapshot`
- `/local/db/migrations/apply` and `/local/db/migrations/rollback` stay unregistered (`daemon_ops_mutating_registered: false`)
- the inspector never opens a database, never curls the daemon, never emits DSNs or executable SQL, and does not register a stock production driver

See [debugging and profiling](../debugging-and-profiling.md) and [development](../development.md). A future production-admin design must not silently rewrite this boundary in place.

## Follow-up options

1. Add crash/restart recovery for pending ground entries only after deciding whether in-memory bootstrap handles should survive process restart at all.
2. Add import/backfill execution tooling only after operators have a closed quarantine/validation policy for retained exports.
3. ~~Add one DB-backed pending ground repository beside tip-`0010` and the read-only export.~~ Done for opt-in `worldruntime.SQLGroundItemStore`: caller-supplied executor, schema preflight through additive `0030`, whole-snapshot replace, export identity stays tip-`0010`. FileStore remains stock process-restart rematerialize. A stock production driver and `gamed` SQL cutover remain out of scope.
4. ~~Keep DB apply/rollback surfaces CLI-only and daemon ops endpoints read-only unless a future production-admin design explicitly changes that boundary.~~ Done — `metin2-migrate apply-boundary` restates the CLI-only apply/rollback contract against the closed read-only loopback ops set; `/local/db/migrations/apply` and `/local/db/migrations/rollback` stay unregistered.
