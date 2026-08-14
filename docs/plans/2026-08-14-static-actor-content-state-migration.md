# Static Actor Content-State Migration — 2026-08-14

## Objective

Freeze the first schema-only database boundary for authored static actors and interaction definitions without changing the shipped runtime away from its current file-backed static actor and interaction stores.

The project already had migration boundaries for account/character roster, item-bearing character state, quest flags, item templates, and auth login-ticket handoff. This slice adds the missing authored-content schema contract for the content seam that currently drives visible/service actors, spawn-backed practice mobs, merchant previews, and warp/info/talk definitions.

## Contract frozen by this slice

The embedded `db/migrations` catalog now includes `0008_static_actor_content_state`.

The `up` migration creates:

- `interaction_definitions`
  - keyed by `(kind, ref)`;
  - accepts the current authored kinds: `info`, `talk`, `warp`, and `shop_preview`;
  - stores text/title/map destination fields with per-kind checks that match the current bootstrap file-store semantics;
  - preserves timestamps as DB-owned metadata for future import/repository work.
- `interaction_merchant_catalog_entries`
  - keyed by `(definition_kind, definition_ref, slot)`;
  - restricted to `shop_preview` definitions;
  - stores item vnum, price, and count with the current bootstrap bounds (`slot < 40`, price within `uint32`, count within `uint8`).
- `static_actors`
  - keyed by stable authored/runtime `entity_id`;
  - stores name, map index, x/y placement, race number, optional spawn home, optional combat profile, optional interaction reference, optional spawn-group ref, and reward experience/gold;
  - constrains actor race numbers to the current client-visible `uint16` range;
  - constrains interaction metadata and spawn-group/combat-profile combinations so spawn-backed actors do not also carry interaction refs.
- `static_actor_reward_drops`
  - keyed by `(entity_id, position)`;
  - stores ordered reward drop item vnums for spawn-backed actors.

The `down` migration drops child tables, indexes, and parent tables in reverse dependency order. The migration is manifest-pinned like the rest of the catalog, so historical SQL drift fails closed.

## What this is not yet

This slice deliberately does not:

- make static actors or interactions DB-backed at runtime;
- add static actor / interaction migration-shaped JSON exports;
- import file-backed content into a database;
- add runtime repository implementations;
- add DB-backed content bundles, combat-profile tables, shop runtime tables, ground items, exchange/trade state, or world runtime persistence;
- choose a production DB driver or enable daemon auto-migration.

The schema is intentionally a contract/backfill target first. File-backed stores remain authoritative for the shipped bootstrap runtime.

## Why this order

Static actors and interaction definitions are now important authored-content inputs, but the DB lane should not jump directly from JSON content bundles to DB-backed runtime repositories. Freezing a schema-only boundary first gives future import/quarantine and repository work a concrete target while preserving the existing local-only content QA and file-backed runtime behavior.

## TDD and validation

Focused coverage in `db/migrations` proves:

- the built-in catalog now includes `0008_static_actor_content_state` after `0007_auth_login_ticket_handoff`;
- the migration has the expected tables, foreign-key boundaries, indexes, and check constraints;
- the manifest pins both new SQL files by SHA-256;
- built-in dry-run planning includes the new eighth pending migration step.

Adjacent runtime/ops/CLI tests were updated only where they assert the current catalog tip, keeping migration status and catalog summaries honest without exposing executable SQL.

Validation for this slice:

- `go test ./db/migrations ./internal/migratecli ./internal/minimal ./internal/ops -count=1`;
- full repo `go test ./... -count=1 -timeout=120s`;
- `go vet ./...`;
- `gofmt -l .`;
- `git diff --check`.

## Follow-up options

1. Add read-only static actor and interaction-definition migration-shaped exports from the committed JSON stores.
2. Add a strict import/quarantine preflight that validates exported content rows against this schema without applying them.
3. Extract a narrow static actor / interaction repository seam only after an export/import test proves the boundary reduces file-state coupling.
4. Keep daemon-local migration endpoints read-only unless a future production-admin design intentionally changes that boundary.
