# Static-Actor PvE Interaction-State Migration — 2026-08-21

## Objective

Widen the authored static-actor / interaction-definition SQL boundary so the playable PvE vertical's `quest_flag` turn-ins, `open_safebox` warehouse NPCs, optional service quest gates, and spawn-group kill-quest credit metadata can target a migration-shaped export/quarantine contract — without making content DB-backed at runtime.

Migration `0008_static_actor_content_state` still only owns `info` / `talk` / `warp` / `shop_preview`. The live file stores and PvE authoring bundle already use `quest_flag` and `open_safebox`, and the `0008` exporter intentionally rejects those kinds. This slice freezes the additive schema + tip export boundary called out after the `0008` quarantine plan.

## Contract frozen by this slice

The embedded `db/migrations` catalog adds `0012_static_actor_pve_interaction_state` after `0011_character_point_state`.

The `up` migration:

1. Rebuilds `interaction_definitions` so `kind` accepts `open_safebox` and `quest_flag` beside the historical four kinds, and adds nullable/defaulted columns already owned by the bootstrap interaction store:
   - `size` (`0..3`, authored `0` means runtime default page count for `open_safebox`)
   - `quest_ref`, `quest_flag`, `quest_from`, `quest_to`
   - `reward_experience`, `reward_gold`, `consume_gold`, `consume_experience`
2. Adds child tables for quest-flag carried-item tables:
   - `interaction_quest_flag_reward_items` keyed by `(definition_kind, definition_ref, position)`
   - `interaction_quest_flag_consume_items` keyed by `(definition_kind, definition_ref, position)`
   - both restricted to `quest_flag` parents, positions contiguous from `0`, max 8 entries, vnum/count bounds matching the bootstrap authoring rules
3. Rebuilds `static_actors` so interaction refs may point at `open_safebox` / `quest_flag`, and adds kill-quest / require-gate columns already owned by the bootstrap static-actor store:
   - `reward_quest_ref`, `reward_quest_flag`, `reward_quest_from`, `reward_quest_to`, `reward_quest_text`
   - `require_quest_ref`, `require_quest_flag`, `require_quest_from`
4. Recreates the historical indexes that `0008` owned for map / interaction / spawn-group lookup after the table rebuild.

The `down` migration reverses those additive tables/columns by restoring the `0008` table shapes (data-preserving for historical kinds; PvE-only rows/columns are dropped on rollback).

Export/quarantine tip for this surface moves from migration `8` / `static_actor_content_state` to migration `12` / `static_actor_pve_interaction_state`, matching the item-template tip pattern (`0005` → `0009`). Retained `migration_version = 8` artifacts fail quarantine closed; operators re-export from the live stores.

`ExportStaticActorContentState` / quarantine accept and project:

- `open_safebox` and `quest_flag` definitions (plus optional service quest gates on the historical service kinds)
- quest-flag reward/consume item child rows
- static-actor kill-quest credit / require-gate fields

File-backed stores remain authoritative. No DB driver selection, import/backfill execution, or daemon-local mutating migration endpoint is added.

## What this is not yet

- SQL-backed static-actor / interaction repositories at runtime
- INSERT / backfill / restore-from-export tooling
- durable safebox persistence / password load
- ground-item restart durability
- combat-profile / aggro / leash SQL ownership
- remote admin API

## TDD and validation

Focused coverage:

- `go test ./db/migrations -run 'BuiltInCatalog|PlanUpToLatestUsesBuiltIn' -count=1`
- `go test ./internal/staticstore -run 'StaticActorContentState|PVEInteraction|MigrationVersion' -count=1`
- `go test ./internal/ops -run 'MigrationStatus|StaticActorContentState' -count=1`
- `go test ./internal/minimal -run 'MigrationCatalog|MigrationStatus|StaticActorContentState' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
2. Optionally project combat-profile / aggro / leash metadata once operators need those columns for offline content review.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
4. Keep daemon-local migration endpoints read-only.
5. ~~Prove the checked-in NPC QA fixture projects onto the `0012` export/quarantine tip.~~ Done: see [NPC service bundle 0012 export quarantine](2026-08-22-npc-service-bundle-0012-export-quarantine.md).
6. ~~Prove the authoring-form PvE vertical fixture exports onto `0012` and that quarantine fails closed without portable `combat_profiles[]`.~~ Done: see [PvE vertical authoring 0012 export combat-profile gap](2026-08-22-pve-vertical-authoring-0012-export-combat-profile-gap.md). Next migration tip should retain combat-profile rows.
