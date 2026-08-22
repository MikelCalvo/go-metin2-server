# Combat-Profile Content-State Migration — 2026-08-22

## Objective

Widen the authored static-actor content SQL / export / quarantine tip so portable
`combat_profiles[]` (formula HP/damage, aggro/leash, retaliation, death-reward
defaults) are retained beside actor `combat_profile` name references — closing
the fail-closed gap frozen by
[PvE vertical authoring 0012 export combat-profile gap](2026-08-22-pve-vertical-authoring-0012-export-combat-profile-gap.md).

Migration `0012_static_actor_pve_interaction_state` still exports only the
profile **name** on `static_actors`. The playable PvE authoring fixture uses
portable `qa_pve_vertical_practice_mob`, so quarantine rebuilds a snapshot
without `combat_profiles[]` and fails closed. This slice freezes the additive
schema + tip export boundary without making combat profiles DB-backed at
runtime.

## Contract frozen by this slice

The embedded `db/migrations` catalog adds `0013_static_actor_combat_profile_state`
after `0012_static_actor_pve_interaction_state`.

The `up` migration:

1. Creates `static_actor_combat_profiles` keyed by canonical lowercase
   snake-case `profile`, storing the portable snapshot scalars already owned by
   `worldruntime.StaticActorCombatProfileSnapshot`:
   - `max_hp`, `damage_per_normal_attack`, `attack_value`, `defense_value`
   - `level`, `rank`, `respawn_delay_ms`
   - `aggro_radius`, `leash_radius` (authored `0` means bootstrap default)
   - `retaliation_point_delta` (authored `0` means bootstrap default `-1`;
     positive values are rejected)
   - `death_reward_experience`, `death_reward_gold`
   - rejects built-in names `practice_mob` / `training_dummy` (those stay
     process builtins, not portable rows)
2. Creates child table `static_actor_combat_profile_death_reward_drops` keyed by
   `(profile, position)` for ordered death-reward drop vnums (contiguous from
   `0`, unique positive vnums per profile, max 255 entries).

The `down` migration drops the child table then the parent table.

Export/quarantine tip for the static-actor content surface moves from migration
`12` / `static_actor_pve_interaction_state` to migration `13` /
`static_actor_combat_profile_state`. Retained `migration_version = 12`
artifacts fail quarantine closed; operators re-export from the live stores.

`ExportStaticActorContentState` / quarantine additionally project:

- `combat_profiles` rows sorted by profile name
- `combat_profile_death_reward_drops` child rows sorted by profile / position

File-backed stores remain authoritative. No DB driver selection, import/backfill
execution, or daemon-local mutating migration endpoint is added.

## What this is not yet

- SQL-backed static-actor / combat-profile repositories at runtime
- INSERT / backfill / restore-from-export tooling
- ground-item restart durability
- weighted/random loot tables or branching quest scripts
- remote admin API
- automatic registration of portable profiles into the process registry from SQL

## TDD and validation

Focused coverage:

- `go test ./db/migrations -run 'BuiltInCatalog|PlanUpToLatestUsesBuiltIn|CatalogSummary' -count=1`
- `go test ./internal/staticstore -run 'StaticActorContentState|CombatProfile|MigrationVersion|Quarantine' -count=1`
- `go test ./internal/contentbundle -run 'PveVerticalAuthoringBundleExports|NPCServiceBundleExportsAndQuarantines' -count=1`
- `go test ./internal/ops -run 'MigrationStatus|StaticActorContentState' -count=1`
- `go test ./internal/migratecli -run 'QuarantineExport' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep import/backfill execution deferred until a driver-backed harness and backup policy exist.
2. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
3. Keep daemon-local migration endpoints read-only.
4. Optional later: project process-registry builtins into a read-only offline review artifact (still not portable store rows).
