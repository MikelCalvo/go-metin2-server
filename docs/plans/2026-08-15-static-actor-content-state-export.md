# Static Actor Content-State Export — 2026-08-15

## Objective

Add a read-only, migration-shaped export for committed authored static actors and interaction definitions at the existing `0008_static_actor_content_state` schema boundary.

The runtime remains file-backed. This slice is an operator/backfill preflight contract: it lets tooling inspect whether the JSON static-actor and interaction-definition stores can be represented by the project-owned SQL schema before any future import/quarantine or repository implementation exists.

## Contract frozen by this slice

`internal/staticstore` now exposes `ExportStaticActorContentState(...)` and `ExportStaticActorContentStateFromStores(...)`.

The export returns:

- `migration_version = 8`
- `migration_name = static_actor_content_state`
- deterministic `interaction_definitions`
- deterministic `merchant_catalog_entries`
- deterministic `static_actors`
- deterministic `reward_drops`

Ordering is stable for future backfill tools:

1. interaction definitions by `kind`, then `ref`;
2. merchant catalog rows by definition order, then catalog slot;
3. static actors in the canonical static-store order already used by the committed JSON snapshot;
4. reward drops by actor order and normalized drop position.

Missing committed static-actor or interaction-definition snapshots export as empty migration-shaped collections. This matches the current bootstrap runtime semantics where missing authored stores are valid empty authored content sets.

## Fail-closed rules

The export validates both source snapshots before returning rows. It rejects:

- invalid static actor snapshots;
- invalid interaction definitions;
- interaction-definition kinds that are valid in the newer file-backed runtime but are not columns/kinds in the historical `0008_static_actor_content_state` migration yet, including `quest_flag`;
- duplicate interaction definition keys;
- static actors whose interaction refs do not exist in the committed interaction-definition snapshot;
- static actors with more than `255` reward drop rows, matching the migration's `position < 255` constraint.

It does not silently coerce dangling refs, newer quest-state trigger definitions, or lossy reward-drop payloads into a future database shape. A future migration/export slice can add `quest_flag` rows explicitly once the DB boundary owns the quest trigger fields.

## Local ops surface

`gamed` now registers:

- `GET /local/static-actors/exports/static-actor-content-state`

The endpoint is loopback-only, read-only, and metadata/payload-only. It returns `409` when the export cannot validate the committed JSON stores. It does not expose executable SQL, open a DB connection, apply migrations, import content, mutate JSON stores, or include runtime-only combat/respawn/target state.

## What this is not yet

This slice deliberately does not:

- make static actors or interactions DB-backed at runtime;
- import the exported rows into a database;
- quarantine exported rows into a staging directory;
- add static-content repository implementations;
- add combat-profile, live HP, respawn timer, combat-target, exchange/trade, ground-item, or world-runtime tables;
- add daemon-local migration apply endpoints.

## Validation

Focused validation for this slice:

- `go test ./internal/staticstore -run TestExportStaticActorContentState -count=1`
- `go test ./internal/staticstore ./internal/minimal ./internal/ops -run 'TestExportStaticActorContentState|TestGameRuntimeStaticActorContentStateExportProjectsCommittedSnapshots|TestLocalStaticActorContentStateExportEndpoint' -count=1`

Full validation before commit:

- `go test ./internal/staticstore ./internal/minimal ./internal/ops -count=1`
- `go test ./... -count=1 -timeout=120s`
- `go vet ./...`
- `gofmt -l .`
- `git diff --check`

## Follow-up options

1. Add JSON-file-store import/quarantine tooling that consumes this export plus the existing account/item/quest/login-ticket migration-shaped exports without applying rows. The static-actor content-state quarantine preflight is now landed in `docs/plans/2026-08-19-static-actor-content-state-quarantine.md`.
2. Add a later migration/export slice that widens the schema for `quest_flag` interaction definitions and kill-quest credit fields once those columns are owned by the DB boundary.
3. Extract a narrow static actor / interaction repository seam only after an export/import test proves it reduces file-state coupling.
4. Add combat-profile migration/export boundaries separately; do not overload this content-state export with runtime combat state.
5. Keep runtime content file-backed until a repository/backfill consumer exists.
