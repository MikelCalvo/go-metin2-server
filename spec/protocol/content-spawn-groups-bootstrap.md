# Content Spawn-Groups Bootstrap

This document freezes the first authored content contract for attackable non-player spawns in `go-metin2-server`.

It sits on top of:
- `combat-training-dummy-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `spawn-leash-bootstrap.md`
- `static-actor-interaction-authoring.md`
- `non-player-entity-bootstrap.md`

Those documents already freeze:
- visible non-player runtime identity and deterministic file-backed authored content seams
- one real `training_dummy` combat loop with owned target, attack, HP, death, and respawn behavior
- authored `combat_profile` metadata for bootstrap combatants
- deterministic import/export for bootstrap authored content bundles

What this document adds is the next narrower question:

**What is the smallest honest authored content shape for loading one attackable non-player spawn from content without pretending that packs, wandering AI, loot tables, or a full mob system already exist?**

## Scope

This contract currently applies only to:
- authored content loaded by the current single-process bootstrap runtime
- one stable top-level authored collection named `spawn_groups`
- stationary non-player combatants only
- one authored `combat_profile` per spawn group
- one authored spawn position on one map per spawn group
- one server-owned respawn lifecycle that recreates the combatant from authored content after death
- one pure runtime leash classification seam that compares current position with the authored spawn position and reports `at_home`, `within_radius`, or `return_required`
- deterministic content import/export and validation before runtime mutation
- idempotent no-op content imports: importing the same canonical bundle already live in the runtime must not tear down/recreate attackable actors, replay replacement fanout, clear selected combat targets, reset runtime-owned HP, or discard pending combat timers
- atomic bootstrap visibility for content-bundle replacement: live sessions receive static-actor replacement visibility only after the full replacement succeeds; successful replacements replay deletes for removed actors before newly imported actor bootstrap bursts, and failed replacement/rollback paths discard all staged delete/add visibility frames instead of leaking partial content to online sessions

This contract does **not** yet claim:
- roaming/wandering/pathing AI beyond the pure spawn-position leash classifier
- pack behaviors or multi-wave encounters
- random loot tables, quest rewards, or corpse interactions
- spawn conditions, timers authored per player, or scripting hooks
- hostility/retaliation logic beyond the already-owned combat loop
- dynamic difficulty, random rolls, or weighted spawn tables
- persistence of live HP/dead state across daemon restart

## Why a spawn-group contract now

The repository already owns a real first combat loop, but the current attackable actor is still effectively seeded through runtime/bootstrap seams.

The next honest step is not “full mobs.”
It is a tiny authored contract that can answer four concrete questions:
- which attackable actor should exist
- where it should appear
- which `combat_profile` should define its combat defaults
- which authored identity should own its death-to-respawn recreation

That is enough to move from a bootstrap-only dummy toward real content runtime without opening AI or gameplay systems the repo does not yet own.

## First authored shape

The first owned content shape is a new top-level bundle collection:
- `spawn_groups`

A spawn group is currently intentionally tiny and can be represented as JSON equivalent to:

```json
{
  "ref": "practice.mob_alpha",
  "name": "Practice Mob Alpha",
  "map_index": 42,
  "x": 1775,
  "y": 2875,
  "race_num": 20350,
  "combat_profile": "practice_mob",
  "reward_experience": 0,
  "reward_gold": 0,
  "reward_drop_vnums": []
}
```

In bundle form, the authored surface is therefore:

```json
{
  "spawn_groups": [
    {
      "ref": "practice.mob_alpha",
      "name": "Practice Mob Alpha",
      "map_index": 42,
      "x": 1775,
      "y": 2875,
      "race_num": 20350,
      "combat_profile": "practice_mob",
      "reward_experience": 0,
      "reward_gold": 0,
      "reward_drop_vnums": []
    }
  ]
}
```

The repository-owned example bundle at `docs/examples/bootstrap-npc-service-bundle.json` now includes one `spawn_groups` practice mob with a deliberately non-zero bootstrap reward descriptor plus kill-quest credit (`quest:first_steps.killed_qa_mob`), and the same fixture closes that credit through the authored `QuestHunter` / `quest:first_steps_kill_turnin` `quest_flag` NPC. That example is intended for local QA of the owned target -> hit -> death -> reward loop, including the selected-killer quest-flag advance after the accepted death edge and the follow-up turn-in clear. A separate authoring-only example at `docs/examples/bootstrap-drop-table-authoring-bundle.json` shows the first fixed reward-table convenience shape for EXP, gold, drop-vnum descriptors, kill-quest credit, and the optional require gate. `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json` shows the same gated kill-quest credit authored alone on a shared `drop_tables` row with empty EXP/gold/drop channels, so validation can prove kill-quest-only table expansion without dummy combat rewards or item templates. `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json` is the matching regen authoring form of that kill-quest-only table (one-count `regen_spawns` + gated kill-quest-only `drop_tables`, no item templates). `docs/examples/bootstrap-regen-authoring-bundle.json` shows the first bootstrap regen-ingestion shape with the same gated kill-quest credit on its fixed reward table, which deliberately canonicalizes to the same one-actor `spawn_groups` contract before runtime import. `docs/examples/bootstrap-combat-profile-formula-bundle.json` is the first formula-first playable QA fixture: it authors one custom `combat_profiles` row (`qa_formula_practice_mob`) with explicit `attack_value` / `defense_value` / `max_hp` plus a profile-default death reward, binds `practice.qa_formula_mob` to that profile, and omits legacy `damage_per_normal_attack` / `level` so validation and import derive `damage_per_normal_attack = max(1, attack_value - defense_value)` and default `level = 1` before materializing the spawn. Multi-count `regen_spawns` authoring now expands through the owned pack-placement contract in `docs/plans/2026-08-23-multi-count-regen-pack-placement-contract-freeze.md` into independent one-actor `spawn_groups` only; pack AI, synchronized respawn, random rectangle placement, weighted/random loot, full legacy combat math, and quest/corpse reward systems remain out of scope.

## Field meanings

The first bootstrap spawn-group contract freezes these fields:
- `ref`
  - stable authored identifier for the spawn group
  - unique within the bundle
  - canonical dotted lowercase identifier made of at least two `[a-z][a-z0-9_]*` segments, for example `practice.mob_alpha`
  - runtime import and static-actor snapshot validation reject non-canonical refs instead of preserving ambiguous authored ownership keys
  - this is the authored identity that future runtime respawn ownership binds to
- `name`
  - required operator-friendly display label
  - may surface in debugging, QA, or future operator tooling
  - blank or whitespace-only values fail bundle validation before runtime mutation
- `map_index`
  - the effective bootstrap map where the combatant should spawn
- `x`, `y`
  - authored world coordinates for the spawn point
- `race_num`
  - the bootstrap non-player class/template identifier already used by static actors
- `combat_profile`
  - optional authored combat metadata selector
  - omitted values canonicalize to the current spawn-group default `practice_mob`
  - `training_dummy` remains supported for legacy/bootstrap static actors and explicit authored use
  - `practice_mob` currently reuses the same compact HP, damage, respawn, HP-percent refresh, and rewardless defaults as `training_dummy` while giving spawn-loaded combatants their own authored profile name
  - bundles may include a matching top-level `combat_profiles` snapshot for non-built-in profile names; canonicalization and runtime import register those profile defaults before validating/importing `spawn_groups`, so portable bundles can carry their authored combat profile and reward defaults without requiring a prior local profile registration step
  - file-backed static-actor snapshots now carry the same canonical `combat_profiles` rows for referenced non-built-in profile names, so a restarted `gamed` process can reload authored formula HP/damage/respawn/retaliation/reward defaults before materializing persisted `spawn_groups` instead of depending on a process-local profile registration that disappeared at daemon exit; hand-edited snapshot rows whose `combat_profiles[].profile` identity is padded, uppercase, dotted, hyphenated, digit-leading, blank, or otherwise non-canonical fail snapshot load/save validation instead of being trimmed or rewritten into a different runtime selector
  - portable `combat_profiles[].profile` identities must already be canonical lowercase snake-case on input; canonicalization and runtime import reject surrounding whitespace, uppercase letters, dots, hyphens, leading digits, blank names, and built-in profile names instead of trimming or rewriting them into a different process-local key
  - if later bundle canonicalization, validation, or static-actor replacement fails after registering new combat profiles, the bootstrap importer/canonicalizer rolls back the profile registrations it introduced for that failed import; already-registered local profiles are left untouched
  - if a bundle carries duplicate `combat_profiles` snapshots for the same profile name, even when the duplicate snapshots are identical, the import fails closed before registering that profile or materializing spawn actors so hand-authored portable bundles cannot race two conflicting/default definitions through the runtime seam
  - portable `combat_profiles` snapshots are validated against the same identity and damage-shape rules as runtime profile registration: omitted `attack_value` with legacy `damage_per_normal_attack` expands to `attack_value = damage_per_normal_attack + defense_value` (failing closed on `uint16` overflow), omitted legacy damage with explicit formula fields expands `damage_per_normal_attack` from `max(1, attack_value - defense_value)`, explicit `damage_per_normal_attack` when present must match that deterministic formula after expansion, and formula damage above `max_hp` fails closed instead of being silently clamped during bundle import
  - if a bundle carries a `combat_profiles` snapshot for a profile name that is already registered locally, the snapshot must exactly match the registered canonical defaults; conflicting snapshots fail closed before spawn actors are materialized so portable bundle imports cannot silently reinterpret authored combatants with different HP/damage/reward defaults
- `reward_experience`, `reward_gold`, `reward_drop_vnums`
  - optional authored death-reward descriptor fields
  - if all reward fields are omitted or zero/empty, bundle canonicalization now applies the selected combat profile's bootstrap death-reward defaults; the built-in `practice_mob` and `training_dummy` profiles remain rewardless, while registered reward-bearing profiles can provide deterministic defaults
  - explicit non-zero reward fields override profile defaults for that spawn group
  - non-empty drop-vnum lists canonicalize into ascending deterministic order across content bundles and file-backed static-actor snapshots
  - every authored reward drop vnum from either `spawn_groups` or bundled custom `combat_profiles` must resolve to one top-level bundled `item_templates` entry; item-shaped reward bundles that omit `item_templates` fail closed before loopback import/runtime mutation instead of relying on an ambient runtime catalog
  - duplicate authored reward drop vnums fail closed in both `spawn_groups` descriptors and bundled custom `combat_profiles[].death_reward.drop_vnums`; bundle canonicalization must not silently deduplicate malformed profile-default reward lists before validation
  - runtime/export paths apply the same rule after expanding registered combat-profile reward defaults onto spawn groups, so item templates referenced only by those profile-default drop lists remain in the exported portable bundle instead of being filtered away as unused
  - EXP/gold-only reward descriptors may still omit `item_templates`, but any non-empty `reward_drop_vnums` list makes the portable bundle self-contained with item templates
  - bundle summaries now expose each spawn group's authored `x`, `y`, and `race_num` placement/template identity alongside its `reward_drop_vnums` list, deterministic `reward_drop_items` list, and optional kill-quest credit fields (`reward_quest_ref`, `reward_quest_flag`, `reward_quest_from`, `reward_quest_to`, `reward_quest_text`, plus optional require-gate fields `require_quest_ref`, `require_quest_flag`, `require_quest_from`), so operators can inspect spawn placement, visual template, resolved item names, stackability, max counts, optional shop buy/sell prices, owned transfer/merchant rejection metadata, direct-use guard metadata (`confirm_when_use`, `quest_use`, `quest_use_multiple`, `applicable`, and `use_reject_message`), and selected-killer quest-flag credit without cross-reading the full bundle sections
  - bundle summaries also expose aggregate reward totals (`reward_experience_total`, `reward_gold_total`, `reward_drop_item_count`) and deterministic `reward_drops` grouped by item vnum with `source_count`, resolved item name, stackability, max count, optional shop buy/sell prices, owned transfer/merchant rejection metadata, and direct-use guard metadata; this gives operators a compact reward audit before importing a candidate bundle
  - each per-map summary row now carries the reward totals contributed by spawn groups on that map (`reward_experience_total`, `reward_gold_total`, `reward_drop_item_count`) alongside static/interactable/spawn occupancy, so an operator can spot overloaded reward zones without expanding the full `spawn_groups` payload
  - import previews now carry signed top-level and per-map reward amount deltas for `reward_experience_total` and `reward_gold_total`, alongside the existing count deltas, so dry-run replacement output shows reward-budget increases and decreases before an import mutates live authored content
  - import previews expose focused loopback-only readers for authored combat/reward/spawn impact: `POST /local/content-bundle/import-preview/combat-profiles/{profile}` returns one exact portable combat-profile delta, `POST /local/content-bundle/import-preview/spawn-groups/{ref}` returns one exact spawn-group delta including resolved reward-drop item metadata, and `POST /local/content-bundle/import-preview/reward-drops/{item_vnum}` returns one exact grouped reward-drop delta for a non-zero item vnum, so operators can audit one high-risk content identity without fetching and filtering the full preview response
  - `POST /local/content-bundle/import-preview/maps/{map_index}` returns one exact per-map delta row for a candidate replacement bundle, so operators can audit one map's static-actor, spawn-group, service-route, and reward-budget impact without fetching and filtering the full preview response
  - import previews now also carry exact per-map `static_actors`, `spawn_groups`, `quest_flag_routes`, `shop_routes`, and `warp_routes` delta rows under `deltas.maps[]`, so an operator can inspect which authored actors, spawn identities, quest-flag NPC triggers, merchant routes, and teleporter routes are added, removed, or changed on each affected map without manually correlating top-level rows with coordinates
  - import previews now also carry exact `deltas.combat_profiles` rows for portable custom combat-profile snapshots that are added, removed, or changed, and the focused exact-profile preview reader returns the same canonical delta shape for one `profile`, so an operator can see HP/damage/formula/presentation/respawn/reward default changes before a replacement bundle is applied
  - `GET /local/content-bundle/combat-profiles/{profile}` is the loopback-only exact-profile reader over the same live exported bundle summary; it accepts only canonical lowercase snake-case profile names, returns one portable custom `combat_profiles[]` snapshot when present, returns `404` when the live exported bundle does not carry that profile, and exists for local QA/introspection rather than gameplay protocol or content mutation
  - non-zero values use the narrow reward contract in `non-player-reward-bootstrap.md` on the accepted killing hit
  - reward data belongs to the authored spawn group and round-trips through content bundles, static-actor snapshots, and runtime import/export; it is not live character persistence by itself
- `drop_tables[].reward_experience`, `drop_tables[].reward_gold`, `drop_tables[].drop_vnums`, optional kill-quest credit fields (`drop_tables[].reward_quest_ref`, `drop_tables[].reward_quest_flag`, `drop_tables[].reward_quest_from`, `drop_tables[].reward_quest_to`, `drop_tables[].reward_quest_text`, plus optional require-gate fields `require_quest_ref`, `require_quest_flag`, `require_quest_from`), and `spawn_groups[].reward_drop_table_ref`
  - authoring-only fixed reward-table convenience fields for candidate bundles
  - a referenced table can now carry the same deterministic descriptor channels as a direct spawn group: EXP, gold, zero or more fixed item-vnum drops, and optionally one kill-quest credit descriptor including its optional require gate
  - canonicalization expands those table fields into the referencing `spawn_groups[].reward_experience`, `reward_gold`, sorted `reward_drop_vnums`, and optional kill-quest credit fields (including require-gate fields), then strips both `drop_tables` and `reward_drop_table_ref` before runtime import/export
  - scalar-only reward tables are valid and do not require `item_templates`; table entries with non-empty `drop_vnums` must still be backed by bundled `item_templates` for every referenced vnum
  - kill-quest credit on a drop table uses the same identity/validation rules as spawn-group kill-quest credit, including optional require-gate completeness; partial kill-quest fields fail closed before expansion
  - a spawn group that already authors any kill-quest field may not also expand a table that carries kill-quest credit; that conflict fails closed instead of silently preferring one source; the checked-in negative fixture `docs/examples/bootstrap-invalid-conflicting-kill-quest-credit-bundle.json` is the preferred `/local/content-bundle/validate` dry-run for that reject without improvising JSON
  - when a require gate is present on a spawn group (directly or after drop-table/regen expansion), the same bundle must also author a writer for that `(require_quest_ref, require_quest_flag)` pair: either a `quest_flag` interaction definition or a kill-quest credit descriptor that writes the same flag; portable `quest_state` seed rows alone are not writers and fail closed
  - focused authoring fixtures such as `docs/examples/bootstrap-drop-table-authoring-bundle.json`, `docs/examples/bootstrap-kill-quest-only-drop-table-authoring-bundle.json`, `docs/examples/bootstrap-kill-quest-only-regen-authoring-bundle.json`, and `docs/examples/bootstrap-regen-authoring-bundle.json` therefore carry a minimal `quest:first_steps.met_guide` `quest_flag` writer alongside their gated kill-quest tables so validation can prove the require-gate loop without importing the full NPC service fixture
  - those two kill-quest-only fixtures are also bound by focused runtime-import twins (`TestGameRuntimeImportsKillQuestOnlyDropTableAuthoringExample` / `TestGameRuntimeImportsKillQuestOnlyRegenAuthoringExample`) so live spawn materialization carries empty combat channels, gated kill-quest credit, and the require gate without relying only on canonicalize / ops validate or inline Go structs
  - the matching combat+kill-quest authoring fixtures `docs/examples/bootstrap-drop-table-authoring-bundle.json` and `docs/examples/bootstrap-regen-authoring-bundle.json` are likewise bound by focused runtime-import twins (`TestGameRuntimeImportsDropTableAuthoringExample` / `TestGameRuntimeImportsRegenAuthoringExample`) so live spawn materialization carries EXP/gold/sorted drop vnums plus gated kill-quest credit and the require gate without relying only on canonicalize / ops validate or inline Go structs
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-orphan-quest-gate-bundle.json` is the matching gated kill-quest-only drop-table authoring shape with the `met_guide` writer deliberately omitted, so `/local/content-bundle/validate` can prove the orphan-require-gate reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-orphan-require-quest-from-bundle.json` is a spawn group with complete kill-quest credit plus orphan `require_quest_from` (both require identities omitted), so `/local/content-bundle/validate` can prove the orphan-`require_quest_from` reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-partial-require-quest-gate-bundle.json` is a spawn group with complete kill-quest credit plus partial require gate (`require_quest_ref` without `require_quest_flag`), so `/local/content-bundle/validate` can prove the partial-require-gate reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-reverse-partial-require-quest-gate-bundle.json` is a spawn group with complete kill-quest credit plus reverse partial require gate (`require_quest_flag` without `require_quest_ref`), so `/local/content-bundle/validate` can prove the reverse-partial-require-gate reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-partial-kill-quest-credit-bundle.json` is a spawn group with incomplete kill-quest credit (`reward_quest_ref` / `reward_quest_flag` / `reward_quest_to` without `reward_quest_text`), so `/local/content-bundle/validate` can prove the partial-kill-quest-credit reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-kill-quest-from-equals-to-bundle.json` is a spawn group with complete kill-quest credit where `reward_quest_from == reward_quest_to`, so `/local/content-bundle/validate` can prove the no-op kill-quest transition reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-partial-drop-table-kill-quest-credit-bundle.json` is a drop-table authoring shape with incomplete kill-quest credit (`reward_quest_ref` / `reward_quest_flag` / `reward_quest_to` without `reward_quest_text`) expanded by a referencing spawn group, so `/local/content-bundle/validate` can prove the partial drop-table kill-quest-credit reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-orphan-service-quest-gate-bundle.json` is the matching gated `shop_preview` service shape with the `met_guide` writer deliberately omitted, so `/local/content-bundle/validate` can prove the orphan service-gate reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-quest-state-seed-alone-gate-writer-bundle.json` is a gated `talk` service backed only by a portable `quest_state` seed row for `quest:first_steps.met_guide` (no `quest_flag` interaction and no kill-quest credit writer), so `/local/content-bundle/validate` can prove the seed-alone-is-not-a-writer reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-partial-service-quest-gate-bundle.json` is a `talk` service with partial quest gate (`quest_ref` without `quest_flag`), so `/local/content-bundle/validate` can prove the partial service-gate reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-reverse-partial-service-quest-gate-bundle.json` is a `talk` service with reverse partial quest gate (`quest_flag` without `quest_ref`), so `/local/content-bundle/validate` can prove the reverse-partial service-gate reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-orphan-service-quest-from-bundle.json` is an ungated `talk` service with orphan `quest_from` (no `quest_ref` / `quest_flag`), so `/local/content-bundle/validate` can prove the orphan service `quest_from` reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-orphan-service-quest-to-bundle.json` is an ungated `talk` service with orphan `quest_to` (no `quest_ref` / `quest_flag`), so `/local/content-bundle/validate` can prove the orphan service `quest_to` reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-dangling-interaction-ref-bundle.json` is a `talk` static actor whose `interaction_ref` is absent from `interaction_definitions`, so `/local/content-bundle/validate` can prove the dangling-interaction-ref reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-unsupported-interaction-kind-bundle.json` is a static actor with unfrozen `quest` interaction metadata plus an owned same-ref `info` definition, so `/local/content-bundle/validate` can prove the unsupported-interaction-kind reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-duplicate-static-actor-bundle.json` is two portable static-actor rows that collide after canonical trimming of `name` / `interaction_kind` / `interaction_ref`, so `/local/content-bundle/validate` can prove the duplicate-static-actor reject without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-reward-drop-without-item-templates-bundle.json` is a spawn group with non-empty `reward_drop_vnums` and no top-level `item_templates`, so `/local/content-bundle/validate` can prove the missing item-template backing reject for item-shaped reward drops without improvising JSON
  - the checked-in negative fixture `docs/examples/bootstrap-invalid-merchant-catalog-without-item-templates-bundle.json` is a `shop_preview` interaction definition with a non-empty structured catalog and no top-level `item_templates`, so `/local/content-bundle/validate` can prove the missing item-template backing reject for merchant catalogs without improvising JSON
  - a completely empty `drop_tables` row (no EXP/gold/drop channels and no kill-quest credit) still fails closed before expansion; the checked-in negative fixture `docs/examples/bootstrap-invalid-empty-drop-table-bundle.json` is the preferred `/local/content-bundle/validate` dry-run for that reject without improvising JSON
  - table expansion is a no-random-roll authoring convenience only; the live runtime still sees the existing fixed reward descriptor and kill-quest credit on the materialized spawn-backed actor
- `regen_spawns[]`
  - authoring-only bootstrap ingestion shape for candidate bundles that are easier to write in regen-like terms before the runtime owns real pack semantics
  - each entry has the same placement, visual, combat-profile, and reward descriptor fields as one `spawn_groups[]` row plus `count` and optional `pack_spacing`
  - **current owned behavior:** `count` may be `1` or an integer in `2..8`
    - `count == 1` keeps the authored `ref` / `name` / `(x,y)` and requires `pack_spacing` omitted or `0`
    - `count` in `2..8` requires `pack_spacing > 0` and expands into exactly `count` ordinary independent `spawn_groups[]` rows
    - member `ref` = `{authored_ref}.m{NN}` (`m01`..`m08`), member `name` = `{trimmed authored name} {i}`
    - deterministic grid offsets from the authored origin: `cols = ceil(sqrt(count))`, member 1 stays at `(x,y)`, later members step by `pack_spacing` along columns then rows
    - shared combat profile / reward / kill-quest fields copy onto every member; synthesized refs must stay unique and canonical against other expanded members, directly authored `spawn_groups[]`, and other regen rows
  - fail closed before runtime mutation when `count` is omitted/`0`/`> 8`, when `count >= 2` lacks positive `pack_spacing`, when `count == 1` carries `pack_spacing > 0`, or when any synthesized member ref is non-canonical or collides
  - checked-in negative fixtures:
    - `docs/examples/bootstrap-invalid-regen-count-bundle.json` (`count = 2` without `pack_spacing`)
    - `docs/examples/bootstrap-invalid-regen-over-max-count-bundle.json` (`count = 9` with `pack_spacing`)
    - `docs/examples/bootstrap-invalid-regen-one-count-pack-spacing-bundle.json` (`count = 1` with `pack_spacing = 100`)
    - `docs/examples/bootstrap-invalid-colliding-regen-member-refs-bundle.json` (authored `spawn_groups` already owns a synthesized `{ref}.m01` member that multi-count expansion would recreate)
  - positive multi-count QA fixture: `docs/examples/bootstrap-multi-count-regen-authoring-bundle.json` (`count = 2`, `pack_spacing = 100`) expands to `.m01` / `.m02` beside the existing one-count regen authoring example
  - that same fixture is also bound by a focused runtime-import twin (`TestGameRuntimeImportsMultiCountRegenAuthoringExample`) so live spawn materialization carries the expanded pack-member refs, grid placement, reward descriptor, and gated kill-quest credit without relying only on the composed PvE vertical suite
  - the one-count combat+kill-quest regen authoring fixture remains covered by `TestGameRuntimeImportsRegenAuthoringExample` beside that multi-count twin
  - omitted `combat_profile` canonicalizes through the ordinary spawn-group default (`practice_mob`)
  - `reward_drop_table_ref` can reference the same fixed authoring-only `drop_tables[]` entries as direct spawn groups, and is expanded before validation/import
  - canonicalization appends each valid expanded regen member to the canonical `spawn_groups[]` collection, then strips `regen_spawns`, `drop_tables`, and `reward_drop_table_ref`
  - runtime import/export and live respawn still see only independent one-actor `spawn_groups[]` descriptors; this shape does not add pack AI, synchronized respawn, shared HP, random rectangle placement, direction, legacy regen timers, or assist linkage
  - the composed QA fixture `docs/examples/bootstrap-pve-vertical-authoring-bundle.json` authors one-count kill-quest `regen_spawns` plus one denser multi-count practice pack (`count = 2`, `pack_spacing = 100` → `practice.qa_pve_vertical_pack.m01` / `.m02`) with gated kill-quest `drop_tables` and the NPC quest loop so operators can validate/import the authoring form end-to-end; its checked-in expanded twin `docs/examples/bootstrap-pve-vertical-canonical-bundle.json` is the byte-canonical runtime form of that same authoring fixture after regen/drop expansion; `docs/examples/bootstrap-npc-service-bundle.json` remains the byte-canonical runtime form of the narrower owned quest loop without the denser pack and is bound by a focused runtime-import twin (`TestGameRuntimeImportsNpcServiceExample`); the narrow ungated kill-quest credit fixture `docs/examples/bootstrap-kill-quest-credit-bundle.json` is likewise bound by `TestGameRuntimeImportsKillQuestCreditExample`
- operator/runtime edits that preserve the same `spawn_group_ref` must preserve the authored `combat_profile`, reward descriptor, and spawn-home position while changing mutable actor presentation/current-placement fields; delete/recreate or bundle replacement remains the explicit way to replace reward metadata or authored home ownership
- when a spawn-backed actor is updated through the generic static-actor edit path without specifying a new combat profile, the runtime keeps the existing spawn-group combat profile instead of downgrading the actor to non-combat static content

## Why call it a group if it is one actor

The pluralized concept is intentional even though the first version is size `1`.

The repository needs one authored identity that owns respawn and future widening to simple packs.
If the first contract were named only as a single actor record, later slices would have to rename the seam just to add a second member.

So the first bootstrap rule is:
- a spawn group currently recreates exactly one stationary combatant
- future slices may widen the *members inside a group*
- the top-level authored identity (`ref`) should not need to change when that widening happens

## Ownership split

This slice freezes a narrow ownership model:

### Spawn group owns
- authored identity (`ref`)
- map placement (`map_index`, `x`, `y`)
- visual/template selection (`race_num`, optional `name`)
- combat-profile selection (`combat_profile`), defaulting to the bootstrap `practice_mob` profile when omitted
- optional death-reward descriptor (`reward_experience`, `reward_gold`, `reward_drop_vnums`) for the deterministic EXP/gold/drop seam documented in `non-player-reward-bootstrap.md`
- optional kill-quest credit descriptor (`reward_quest_ref`, `reward_quest_flag`, `reward_quest_from`, `reward_quest_to`, `reward_quest_text`, plus optional `require_quest_ref` / `require_quest_flag` / `require_quest_from`) for one fail-closed selected-killer quest-flag transition after the accepted death edge; see `quest-state-bootstrap.md`

### Combat profile owns
- combat defaults and rules shared by authored actors using that profile
- for the current bootstrap profiles, that includes the existing training-dummy HP/death/respawn semantics already frozen elsewhere
- the current profile-default seam is deliberately compact: `max_hp`, `damage_per_normal_attack`, `attack_value`, `defense_value`, descriptor-only `level`, descriptor-only `rank`, `respawn_delay`, `retaliation_point_delta`, and the reward descriptor documented in `non-player-reward-bootstrap.md`
- `attack_value` / `defense_value` are now profile-owned authored stat defaults used by the first deterministic registered-profile damage formula (`max(1, attack_value - defense_value)`); `damage_per_normal_attack` remains the legacy fallback used to preserve older bootstrap profile behavior, legacy-damage profiles that omit `attack_value` canonicalize it as `damage_per_normal_attack + defense_value` during registration, content-bundle canonicalization, and file-backed static-actor snapshot load/save (failing closed when that sum overflows the current `uint16` carrier), formula-first profiles that omit `damage_per_normal_attack` canonicalize that legacy fallback from the same attack/defense formula on those same seams, and profiles whose explicit formula damage would exceed `max_hp` fail closed instead of being silently capped
- `level` / `rank` are now profile-owned metadata for later mob presentation, reward, or formula slices: built-in `training_dummy` and `practice_mob` default to `level = 1` and `rank = 0`, registered profiles preserve explicit values, omitted registered-profile `level` canonicalizes to the same bootstrap level `1`, and omitted registered-profile `rank` remains `0`
- runtime static-actor snapshots now expose the resolved profile combat defaults as `combat_max_hp`, `combat_normal_damage`, `combat_attack_value`, `combat_defense_value`, `combat_level`, and `combat_rank`, plus a non-default `retaliation_point_delta` when the resolved profile uses a custom negative owner-retaliation amount, so loopback introspection, map/visibility snapshots, and later presentation/hostility slices can inspect the effective defaults without re-resolving the profile name; imported spawn actors and later generic static-actor updates preserve those effective fields while keeping the same `spawn_group_ref` / `combat_profile`; current HP mutation, reward payout, target carriers, and respawn timing still use the shared profile registry as the authoritative source rather than these debug snapshot copies, and an omitted `retaliation_point_delta` in these runtime snapshots means the bootstrap default `-1` point-loss applies
- runtime code now has a narrow registration seam for additional bootstrap combat profiles with those same defaults, so later authored profiles can be introduced without hard-coding every new name into target/attack/respawn validation
- registered profile defaults are used by the same shared-world target/attack/death/respawn loop as built-in profiles: target selection starts from the registered `max_hp`, accepted normal attacks apply the registered attack/defense formula, HP percent is derived from that registered max, spawn-backed deaths can resolve the registered profile's reward descriptor, the dead timer uses the registered `respawn_delay`, and the rebuild restores the actor to the registered full HP
- registered profile defaults now also own the first optional deterministic owner-side retaliation amount for spawn-backed practice mobs: omitted or `0` `retaliation_point_delta` canonicalizes to the current bootstrap `-1` HP decrement, negative values are preserved for both the immediate hit-triggered tick and delayed server-origin cadence, and positive values fail closed because this bootstrap retaliation seam cannot heal or buff the owner
- when an existing static/spawn actor is explicitly updated to a different combat profile while its old runtime combat instance is already dead, the update cancels the old pending respawn timer, clears the dead HP state, and makes the updated actor immediately targetable as a fresh live snapshot at the new profile's full HP; same-profile presentation or placement edits keep using the ordinary update/target-clear rules without inventing a new respawn timer
- registered profile names also use the same first aggro-lite ownership and retaliation gate as built-in spawn-backed practice mobs: once the first owner lands an accepted hit in the current live loop, fresh third-party `TARGET` attempts fail closed with the explicit runtime reason `target_engaged` until the existing engagement reset boundaries release or rebuild that actor, and accepted live owner hits plus the delayed server-origin cadence emit the same self-only `GC POINT_CHANGE` HP decrements as the built-in bootstrap practice profiles; an accepted non-zero retarget by the engaged owner cancels the abandoned target's session-local pending delayed beat but does not release that current-life engagement to third-party target selection, while an explicit client `TARGET(0)`, owner disappearance/rebootstrap, owner zero-HP stale cleanup, actor update/removal, or mob death/respawn still releases or rebuilds the ownership boundary; a recorded owner that has already reached the bootstrap `0`-HP floor is treated as stale engagement ownership on the next fresh third-party `TARGET` attempt, so a still-live mob can be reacquired without waiting for its own death / respawn cycle
- registered profile names are immutable for the lifetime of the current process: registration fails closed when the name is blank, has non-canonical surrounding whitespace, is not a lowercase ASCII snake-case identifier (`[a-z][a-z0-9_]*`), names a built-in bootstrap profile, already exists, has neither a legacy `damage_per_normal_attack` value nor an explicit formula `attack_value`, supplies both legacy damage and explicit formula values that disagree after canonicalization, has invalid HP/formula/respawn defaults after canonicalization, has `respawn_delay_ms` outside the positive range that can round-trip safely to the runtime `time.Duration` respawn timer, has effective `damage_per_normal_attack > max_hp`, has explicit formula damage greater than `max_hp`, carries a positive `retaliation_point_delta`, or carries an invalid reward descriptor
- `gamed` exposes a loopback-only operator profile endpoint for process-local profile authoring and inspection:
  - `GET /local/static-actor-combat-profiles`
  - returns a JSON object whose `profiles` field is the deterministic sorted list of built-in and registered combat-profile defaults, including derived `damage_per_normal_attack`, formula stats, presentation metadata, `respawn_delay_ms`, non-default `retaliation_point_delta`, and cloned/sorted reward descriptors using stable snake-case `death_reward` JSON keys (`experience`, `gold`, `drop_vnums`)
  - `POST /local/static-actor-combat-profiles`
  - JSON fields: `profile`, `max_hp`, optional `damage_per_normal_attack`, optional formula fields `attack_value` / `defense_value`, optional presentation `level` / `rank`, `respawn_delay_ms`, optional `retaliation_point_delta`, and optional `death_reward` with `experience`, `gold`, and `drop_vnums`
  - success returns the canonicalized profile defaults, including derived `damage_per_normal_attack`, effective `retaliation_point_delta`, and sorted/deduplicated reward drops
  - request bodies are bounded to 4 KiB; oversized bodies return `413`, and invalid UTF-8 is rejected before JSON decoding or profile registration
  - content-bundle canonicalization and the file-backed static-actor store now snapshot custom registered profiles referenced by `spawn_groups` and `static_actors` in the top-level `combat_profiles` array, including formula stats, presentation metadata, respawn delay, retaliation point delta when it differs from the bootstrap default, and death-reward defaults, so exported authored combat content and persisted spawn state are self-describing for local QA/restart instead of depending only on process-local profile registration state
  - built-in profile names are intentionally omitted from `combat_profiles` because their defaults are runtime-owned bootstrap constants, while custom profiles used by either authored collection are deduplicated and sorted by profile name
  - invalid JSON, unknown fields, non-loopback callers, built-in/duplicate/invalid profile names, profile names with surrounding whitespace, invalid formula defaults, invalid respawn delay, and invalid reward descriptors fail closed without registration
- that registration seam is still process-local operator tooling; content-bundle import/export now carries deterministic `combat_profiles` snapshots for custom authored profiles, but runtime still rejects malformed profile definitions and never canonicalizes padded profile names into a different key
- content-bundle import compares custom `combat_profiles` against existing process-local profile defaults after applying the same canonical formula/default expansion used at registration time, so a formula-first profile that omits `damage_per_normal_attack`, `level`, or `retaliation_point_delta` can be reimported idempotently while conflicting definitions still fail closed. Successful canonical bundle responses and exported summaries now carry the derived `damage_per_normal_attack` and defaulted `level` for those formula-first profiles, keeping portable content self-describing across validation, import, export, and restart/reconnect QA.
- dots, spaces, hyphens, uppercase letters, and leading digits are intentionally rejected for combat profile names so profile identifiers stay distinct from authored `spawn_group_ref` values such as `practice.mob_alpha` and remain safe to compare as stable runtime selectors

### Runtime owns
- live entity IDs / VIDs
- current HP, dead/live state, and pending respawn bookkeeping
- the act of removing and recreating the visible runtime actor after death
- detecting no-op content-bundle imports before replacement, so live combat snapshots keep their current runtime-owned state when authored content has not changed

This means respawn remains **server-driven runtime behavior**, but the runtime now knows *what to recreate and where* because the authored spawn group owns that identity and placement.

## Respawn rule for the first content-loaded combatant

The first spawn-group contract keeps respawn deliberately narrow:
- death still follows `non-player-death-respawn-bootstrap.md`
- respawn is still server-driven, not client-requested
- the recreated actor returns at the preserved authored spawn-group position, even if a prior runtime/operator update moved the materialized actor's current position away from that home
- the recreated actor uses the authored `combat_profile`, or the default bootstrap `practice_mob` profile when the authored group omits that field
- the live runtime actor after respawn is a fresh instance of the same authored spawn group, not persistence resurrecting an old runtime entity ID
- respawn visibility fanout is calculated from the old dead runtime position to the authored-home position: old-only viewers receive `CHARACTER_DEL`, new-only viewers receive the normal add/info/update burst, and retained viewers receive the usual delete-plus-readd refresh
- if the respawn timer is already due before a new player enters the game, the runtime flushes that respawn before building the entering session's static-actor visibility bootstrap, so the fresh client sees the authored spawn group as live/full-HP without a stale `DEAD` replay or a later duplicate respawn rebuild
- for the shipped file-backed `gamed` runtime, the authored-home respawn position is written back to the static-actor snapshot before the live actor is revived; a failed write leaves the dead/displaced actor and due respawn timer intact for retry instead of partially reviving runtime state or queuing visibility frames
- same-profile runtime/operator edits made while a spawn-backed actor is dead may move the materialized snapshot for inspection, but they do not convert that dead return-required actor into an automatic return-step candidate; the pending respawn timer stays the only server-owned lifecycle action until it rebuilds the actor at authored home
- a successful respawn now also treats the rebuilt actor as a fresh combat intent boundary: any stale selected target, pending retaliation timer, or aggro-lite owner state for that actor is scrubbed, affected selected sessions receive one self-only `TARGET(0, 0)` clear before their respawn rebuild frames, and later attacks still require fresh `TARGET` selection

What is **not** yet frozen here:
- per-group custom respawn delays
- conditional spawn windows
- pack-wide synchronized respawn
- scripted on-death / on-respawn hooks

## Validation rules

The first content contract should fail closed when:
- `ref` is empty, duplicated, or not in the canonical dotted lowercase form `[a-z][a-z0-9_]*(.[a-z][a-z0-9_]*)+`
- `name` is empty after trimming whitespace, contains an embedded NUL byte, or is not valid UTF-8
- `map_index` is `0`
- `race_num` is `0` or larger than the current bootstrap visibility packet field can encode (`uint16`, max `65535`)
- static-actor and spawn-group bundle validation share that same fail-closed `race_num` range because both are projected through the owned `CHARACTER_ADD` bootstrap visibility family
- `combat_profile` is unknown when provided; an omitted profile is canonicalized to the bootstrap `practice_mob` profile for this first one-spawn-profile contract
- a non-built-in `combat_profile` is referenced without a matching top-level `combat_profiles` snapshot and no matching profile is already registered locally
- a `combat_profiles` snapshot is unreferenced by any authored static actor or spawn group, duplicates another snapshot after trimming its profile name, names a built-in profile, has a blank/non-canonical profile identity, carries conflicting legacy/formula damage values, has formula damage above `max_hp`, or carries invalid HP/formula/respawn/reward defaults
- when a spawn group omits explicit reward fields and references a custom bundled combat profile, canonicalization applies that profile snapshot's death-reward defaults so import, export, and loopback POST validation all share the same deterministic reward descriptor; if those defaults include drop vnums, the same bundle must also carry matching `item_templates`
- coordinates are malformed for the current bundle schema
- reward scalar values overflow the current bootstrap point-change carrier, or `reward_drop_vnums` contains `0` or duplicate drop vnums
- authoring-only `drop_tables` are malformed: `ref` must use the same canonical dotted lowercase identity rule as `spawn_groups.ref`, a table must carry either at least one non-zero/non-empty combat descriptor channel (`reward_experience`, `reward_gold`, or `drop_vnums`) or a complete kill-quest credit descriptor (empty combat rewards alone remain invalid), scalar values must fit the current bootstrap point-change carrier, `drop_vnums` must be non-zero, duplicate-free, and backed by bundled `item_templates`, optional kill-quest fields must be complete when any are present, every table must be referenced by a `spawn_groups[].reward_drop_table_ref`, and each referencing spawn group must omit explicit `reward_experience`, `reward_gold`, and `reward_drop_vnums` so table expansion cannot silently conflict with a direct descriptor; a spawn group that already authors kill-quest credit also cannot expand a table that carries kill-quest credit
- authoring-only `regen_spawns` are malformed: `ref` follows the same canonical dotted lowercase identity rule and uniqueness constraints after expansion as `spawn_groups.ref`, `name` / placement / `race_num` / optional `combat_profile` use the same validation rules as `spawn_groups`, `count` must be `1` or `2..8` with the owned `pack_spacing` rules from `docs/plans/2026-08-23-multi-count-regen-pack-placement-contract-freeze.md`, and any reward descriptor or `reward_drop_table_ref` must satisfy the same fixed reward rules as a direct spawn group

Import should reject malformed spawn groups before mutating live runtime state. The bundle canonicalization path now keeps spawn-group names explicit instead of synthesizing them from `ref`, rejects duplicate or non-canonical `ref` values without trimming the authored identifier into a different key, and preserves the prior authored/runtime snapshot when validation fails.

Runtime static-actor snapshots are also part of this contract because export, persistence rollback, map/visibility introspection, and respawn/rebuild code all round-trip through the same snapshot shape. A materialized spawn-group actor must therefore preserve its authored `spawn_group_ref` and normalized `combat_profile` in the live runtime snapshot, not just in the initial content-bundle record or file-backed store. When that `combat_profile` is non-built-in, the file-backed static-actor snapshot also preserves the canonical matching `combat_profiles[]` definition and reloads it before actor materialization on restart.

## Content bundle operator/runtime boundary

The bootstrap content-bundle surface uses the same top-level `spawn_groups` collection for export and import through the local operator bundle endpoint.

Current runtime rules:
- spawn-backed live actors export as `spawn_groups`, not as ordinary `static_actors`
- candidate bundles may include a narrow authoring-only `drop_tables` collection for fixed reward descriptors plus optional kill-quest credit; canonicalization expands `spawn_groups[].reward_drop_table_ref` into direct `reward_experience`, `reward_gold`, deterministic sorted `reward_drop_vnums`, and optional kill-quest credit fields, strips `drop_tables` and `reward_drop_table_ref` from the canonical bundle, and leaves runtime/import/export behavior on the existing fixed reward / kill-quest seams instead of adding randomized loot-table execution
- candidate bundles may include a narrow authoring-only `regen_spawns` collection for one-count or multi-count regen-style authoring; canonicalization expands valid entries into ordinary independent `spawn_groups` (one-count keeps the authored ref; `count` in `2..8` synthesizes `{ref}.m{NN}` members on a deterministic `pack_spacing` grid), rejects omitted/`0`/`>8` counts and invalid `pack_spacing`, strips `regen_spawns` before runtime import/export, and leaves live behavior on the existing spawn-group lifecycle instead of adding pack AI / synchronized respawn / random rectangle placement
- exported spawn-group `map_index`, `x`, and `y` come from the preserved authored spawn home when present, not from a displaced materialized current position; older snapshots without `spawn_home` fall back to their current actor position for compatibility
- importing a bundle with `spawn_groups` materializes one runtime static actor per group with the authored `spawn_group_ref`
- the imported actor uses the authored placement, `race_num`, and normalized `combat_profile`
- if the candidate bundle canonicalizes to the exact same content bundle currently exported by the runtime, import is treated as a no-op: the runtime returns the canonical bundle without rewriting item templates, interaction definitions, static actors, live combat HP, selected-target ownership, pending respawn/retaliation state, or queued visibility fanout; the no-op still prunes unrelated stale pending return-step deadlines while preserving still-valid live spawn-group deadlines
- when a successful import materializes a spawn-backed actor while players are already online, the runtime enqueues the normal static-actor visibility bootstrap burst (`CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`) only to sessions that currently share the actor's visible world/AOI
- when a successful import replaces a previously visible static actor, the runtime first enqueues the old actor's `CHARACTER_DEL` and then enqueues newly imported actor bootstrap bursts to sessions that currently share those actors' visible world/AOI
- after a successful replacement commits, pending automatic return-step deadlines are reconciled against the live imported actor set: schedules for removed actors and otherwise stale IDs are pruned, while any still-live replacement actor that already classifies `return_required` keeps or arms its own deadline through the ordinary spawn-return scheduling rule
- sessions outside the configured visibility policy do not receive the imported spawn actor until a later enter-game or AOI/transfer visibility rebuild makes it visible
- before mutating the runtime replacement set, import must be able to export the current canonical content bundle used for rollback; if that preflight export fails after a bundle temporarily registered process-local combat profiles, those imported profiles are rolled back before the error is reported
- bundle canonicalization now rejects a portable `combat_profiles` snapshot when its `profile` name already resolves to a registered process-local profile with different canonical defaults; matching snapshots remain idempotent, but conflicting snapshots fail closed before runtime import can mutate content or profile state
- if static-actor persistence fails after interaction definitions have already been replaced, import fails closed and restores the previously exported content bundle before reporting failure; online sessions do not receive staged delete/add visibility frames for content that failed to commit
- if bundle replacement fails after removing a previously selected or return-required spawn-backed actor, rollback restores the prior actor plus its runtime combat snapshot, selected-target ownership, session-local delayed-retaliation timer, HP, respawn metadata, pending return-step deadline, and reward descriptor state before reporting failure; the waiting session must not receive a staged `CHARACTER_DEL`, imported actor burst, or `TARGET(0, 0)` clear for content that never committed, any restored delayed-retaliation beat remains tied to the same target/snapshot version, and any restored pending return-step deadline remains able to drive the next due capped return through the normal flush path
- the operator endpoint remains loopback-only bootstrap tooling; it is not a gameplay packet or public API

This keeps authored attackable spawn content distinct from hand-authored visible/static actor content while still letting local QA export and re-import the current bootstrap content bundle deterministically, including live QA sessions that are already connected when the import succeeds.

## Local spawn-group introspection

The shipped `gamed` runtime also exposes a loopback-only read model for the currently materialized spawn-backed actors:

- `GET /local/content-bundle/maps/{map_index}/spawn-groups`
- `GET /local/content-bundle/maps/{map_index}/reward-drops`
- `GET /local/spawn-groups`
- `GET /local/spawn-groups/{entity_id}`
- `GET /local/spawn-groups/by-ref/{spawn_group_ref}`
- `GET /local/maps/{map_index}/static-actors`
- `GET /local/maps/{map_index}/spawn-groups`
- `GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>`
- `GET /local/maps/{map_index}/static-actor-respawns`
- `GET /local/maps/{map_index}/combat-targets`
- `GET /local/spawn-group-return-steps`
- `GET /local/spawn-group-return-steps/{entity_id}`
- `GET /local/visibility`
- `POST /local/relocate-preview`
- `POST /local/transfer`

This is an operator/debugging surface, not a gameplay packet and not a content mutation API.

`GET /local/content-bundle/maps/{map_index}/spawn-groups` returns the deterministic authored-content `spawn_groups[]` rows for one map from the live exported content-bundle summary. It rejects malformed or zero map indexes with `400`, returns `404` when the exported bundle has no authored row for that map, and returns an empty JSON array for a known authored map with no spawn groups. This is the bundle-summary counterpart to the live runtime map-spawn endpoint; it reads authored placement/reward definitions, not current HP/death/leash state.
`GET /local/content-bundle/maps/{map_index}/reward-drops` returns deterministic map-local aggregate `reward_drops[]` rows for item-shaped rewards contributed by spawn groups on that map. It shares the same loopback-only, read-only, malformed-map, missing-map, and empty-array semantics as the map spawn-group reader. The row shape matches global summary `reward_drops[]`, but `source_count` is recomputed from only that map's `spawn_groups[].reward_drop_vnums`, so QA can audit one map's reward item exposure without expanding every spawn group.
`GET /local/spawn-groups` returns the deterministic global subset of static-actor snapshots whose `spawn_group_ref` is non-empty.
It intentionally omits ordinary `static_actors`, even if they have a combat profile, so local QA can distinguish authored attackable spawn presence from hand-authored visible/service actors without fetching and filtering the full `/local/static-actors` list.
`GET /local/spawn-groups/{entity_id}` returns the same snapshot shape for one currently materialized spawn-backed actor, using the runtime entity ID / client-visible static-actor `VID` as the path key.
It returns `404` when the entity is missing or belongs to an ordinary non-spawn static actor, preserving the boundary that this endpoint is for authored `spawn_groups` only.
`GET /local/spawn-groups/by-ref/{spawn_group_ref}` returns the same snapshot shape by the authored dotted lowercase `spawn_group_ref` identity.
It fails closed with `400` for malformed or non-canonical refs, including refs with surrounding whitespace after URL decoding, and returns `404` when the ref is well-formed but not materialized in the current runtime.
If runtime state ever contains more than one materialized actor with the same authored ref, the by-ref lookup also returns `404` instead of choosing an arbitrary duplicate; authored `spawn_group_ref` is intended to be unique, and ambiguous runtime ownership must fail closed.
This gives local QA a stable authored-content lookup without first discovering the current runtime entity ID, while preserving entity-ID lookup for client-visible `VID` debugging.
`GET /local/spawn-groups/{entity_id}/leash?radius=<positive-int>` returns a read-only spawn-leash classification for one materialized spawn-backed actor. The response embeds the same `actor` row plus `home`, `current`, `radius`, `status`, `return_required`, and optional `return_target`. The `home` fields stay anchored to the authored `spawn_groups` placement while `current` reflects the materialized actor position at lookup time, so local QA can confirm `within_radius` or `return_required` after a runtime/operator position edit without mutating actor state.
`GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>` returns that same read-only leash-classification shape for every materialized spawn-backed actor currently on one effective runtime map. It is the map-local companion to `/local/maps/{map_index}/spawn-groups`, rejects malformed/zero map indexes and missing/non-positive radii with `400`, returns `404` for unknown maps, and returns an empty array for a known map whose current static-actor occupancy contains no spawn-backed actors.
`POST /local/spawn-groups/{entity_id}/return-step?max_step=<positive-int>` is the loopback-only one-step return trigger. It accepts no request body, plans one capped step with the default leash radius, persists the materialized actor at `step.next` before mutating runtime state, preserves HP/death/reward/combat metadata, reuses the ordinary static-actor visibility transition path, returns `{actor,step}`, and no-ops without persistence/frames for actors that are already `at_home` or `within_radius`. When a step actually moves the actor, it releases current practice-mob engagement and clears selected combat targets bound to that actor's visible `VID`, so stale selected attacks and delayed retaliation beats fail closed until a fresh target/engagement starts. If that manual/operator step leaves the actor still `return_required`, it also replaces any older pending automatic deadline with a new one-second return-step deadline measured from the manual step time, so the previous pre-manual due time cannot fire immediately after the operator moved the actor. The first server-owned executor now reuses that same one-step path from the pending server-frame flush loop with a fixed `max_step = 100` and one-second re-arm while the actor remains `return_required`; runtime startup now also arms that same executor for live persisted spawn-backed actors restored already outside leash. A due server-owned step whose static snapshot persistence fails emits no visibility frames, leaves the materialized actor unchanged in runtime and persistence, and retries on a later one-second deadline while the actor still reports `return_required`. This is still a QA/recovery bridge toward later chase/return AI, not final mob movement.
`GET /local/spawn-group-return-steps` and `GET /local/spawn-group-return-steps/{entity_id}` are the paired read-only pending-return schedule surface for that server-owned executor. Each row reports `entity_id`, `ready_at`, `remaining_ms`, the current return-required spawn-group `actor`, and the planned `step` shape that would be applied by the next due capped return step. Rows are deterministic by entity ID, clamp already-due but unflushed timers to `remaining_ms = 0`, omit stale schedules whose actors are missing/dead/no longer return-required, and return `404` for exact lookups that are absent or no longer safe to step.
`POST /local/spawn-groups/{entity_id}/return-home` is the paired loopback-only controlled exact-home return trigger. It accepts no request body, restores one live spawn-backed actor to preserved authored home, fails closed for missing/non-spawn/dead actors or failed static snapshot persistence when a coordinate write is required, preserves HP/death/reward/combat metadata, releases current engagement/selected-target ownership for that actor, clears any pending automatic return-step deadline for that actor, and reuses ordinary static-actor visibility deltas so old-position viewers receive `CHARACTER_DEL`, home-position viewers receive the normal add/info/update burst, and retained viewers receive delete-plus-readd at home before target-clear frames. If the actor is already exactly at authored home, the trigger skips the no-op static snapshot write and still performs the lifecycle reset even when the static snapshot store is temporarily unavailable. Removing a materialized spawn-backed actor also clears any pending automatic return-step deadline for that entity ID after the removal commits.
`GET /local/maps/{map_index}/static-actors` returns the deterministic full static-actor subset for one effective map, including both ordinary service actors and spawn-backed actors, without requiring callers to fetch the full `/local/maps` occupancy row or filter the global `/local/static-actors` list.
`GET /local/maps/{map_index}/spawn-groups` returns the deterministic spawn-backed subset for one effective map without requiring callers to fetch the full `/local/maps` occupancy row or filter the global `/local/spawn-groups` list.
`GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>` returns the deterministic spawn-backed subset plus current leash classification for one effective map without requiring callers to fan out exact entity leash queries themselves.
`GET /local/maps/{map_index}/static-actor-respawns` returns the pending server-driven respawn timers whose dead actor currently belongs to one effective map, using the same `entity_id`, `ready_at`, `remaining_ms`, and dead static-actor row shape as `/local/static-actor-respawns`.
`GET /local/maps/{map_index}/combat-targets` returns the active selected combat-target snapshots whose selected subject currently belongs to one effective map, using the same subject/target/HP/damage/engagement row shape as `/local/combat-targets`. That read-only subset now mirrors the current gameplay target gates too: out-of-range selections, `return_required` spawn actors, aggro-lite blocked third-party selections, dead targets, and other unresolved runtime combat states are omitted instead of appearing as active rows.
These map-scoped endpoints are loopback-only like the adjacent map/spawn-group inspection endpoints, reject malformed or zero map-index path values with `400`, return `404` when the runtime cannot resolve that map-scoped snapshot, and return an empty JSON array when the map is known but has no actors, leash rows, respawns, or active target selections in the requested subset.
Rows use the same static-actor snapshot shape and ordering as the flat static-actor and spawn-group lists.
`GET /local/visibility` now carries the same subset per connected character as `visible_spawn_groups` beside the full `visible_static_actors` list.
That per-player subset obeys the same topology/AOI visibility policy as `visible_static_actors`; actors outside the subject's visible world are omitted, and runtime-owned dead practice mobs keep `dead: true` in both arrays while they are waiting for server-driven respawn.
The broader map-occupancy view also carries that same per-map subset:

- `GET /local/maps`
  - each map entry keeps the full `static_actors` array unchanged
  - each map entry also exposes `spawn_group_count` and `spawn_groups` for the spawn-backed actors on that effective map
  - `spawn_groups` rows use the same static-actor snapshot shape as `GET /local/spawn-groups`

This makes authored attackable spawn placement visible in map-local debugging output while preserving the existing static-actor occupancy contract for all visible actors.

The structured relocation preview/commit surfaces also expose spawn-backed visibility subsets beside their full static-actor visibility arrays:

- `current_visible_spawn_groups`
- `target_visible_spawn_groups`
- `removed_visible_spawn_groups`
- `added_visible_spawn_groups`

Those fields are deterministic filters of the matching `*_visible_static_actors` arrays by non-empty `spawn_group_ref`.
They preserve the same static-actor snapshot fields as `/local/spawn-groups`, including `dead: true` during the runtime-owned respawn interval and reward descriptor fields for authored reward mobs.

Each row reuses the current static-actor snapshot fields:
- `entity_id` / client-visible static-actor `VID`
- `name`
- effective `map_index`
- `x`, `y`
- `race_num`
- resolved `combat_profile`
- resolved combat/profile metadata (`combat_max_hp`, `combat_normal_damage`, `combat_attack_value`, `combat_defense_value`, `combat_level`, `combat_rank`)
- `spawn_group_ref`
- reward descriptor fields (`reward_experience`, `reward_gold`, `reward_drop_vnums`)
- optional `dead: true` while the actor is in its runtime-owned dead interval before respawn

Rows are sorted by actor name with `entity_id` as the tie-breaker, matching the existing static-actor snapshot ordering.
This keeps spawn content observable directly through the runtime while preserving the existing export/import bundle contract as the authoring source of truth.

## Relationship to existing static actors

This document does **not** retroactively make every static actor attackable.

The intended separation is:
- `static_actors` remain the authored seam for visible world actors that may also carry interaction metadata
- `spawn_groups` become the authored seam for runtime-owned attackable combat spawns

A future actor might visually resemble a static actor, but attackable respawn-owned content should load through `spawn_groups`, not by treating every bootstrap static actor as a hidden mob.

## First owned multi-map / reconnect anti-leak matrix

Question frozen here:

**Once same-map chase / return-step MOVE and due-respawn EnterGame preflight already exist, what is the smallest explicit anti-duplicate / anti-resurrect contract that keeps content-loaded spawn groups honest across reconnect, fresh EnterGame, and multi-map membership without inventing a second AI scheduler or cross-map return MOVE?**

Contract for Track A item 6:

1. **Still-dead EnterGame / reconnect bootstrap**
   - if a content-loaded spawn-group combatant is still inside its server-owned dead interval when a nearby client enters or reconnects, that client receives the ordinary `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` burst followed by one trailing `GC DEAD(vid)` replay
   - the still-dead actor stays non-targetable / non-attackable
   - EnterGame must **not** flush that respawn early; only an already-due respawn timer may preflight into a live full-HP bootstrap without a stale `DEAD` replay

2. **One authored ref, one live runtime actor**
   - reconnect / reclaim / fresh EnterGame never rematerializes a second actor for the same authored `spawn_group_ref`
   - if runtime state ever contains more than one materialized actor for the same authored ref, exact by-ref lookup continues to fail closed (`404`) instead of choosing an arbitrary duplicate

3. **Map / AOI scoped content visibility**
   - successful content-bundle import / replacement visibility remains map- and AOI-scoped: sessions on another map receive no spawn add/delete frames for that replacement
   - failed replacement / rollback paths still discard all staged visibility frames

4. **Cross-map return-home / return-step membership**
   - cross-map return-home / return-step stays outside MOVE choreography and keeps delete/readd / direct-home rebuild; `spawn-leash-bootstrap.md` now freezes that delete/readd packet boundary as the Track A bootstrap answer (`CHARACTER_DEL` on origin, ordinary add/info/update on home, no invented cross-map `MOVE` / `GC WARP`)
   - a successful cross-map return restores exactly one entity to the authored home map and must leave no dual-map occupancy or duplicate `spawn_group_ref` membership behind

5. **Leave / transfer ownership cleanup**
   - owner Leave / logout / close, phase-select leave, EnterGame reclaim that drops stale engagement ownership, owner death floor, client-originated `TARGET(0)` clear-target, and owner transfer/warp away clear engagement / selected-target / pending chase ownership without resurrecting dead combat state or inventing a second spawn instance
   - due return-step and due chase-step EnterGame / transfer preflights remain the owned anti-stale-position path and must not emit a later duplicate queued rebuild for the same due timer

6. **Still-dead content-bundle replacement anti-resurrect**
   - while a content-loaded spawn-group combatant is still inside its server-owned dead interval, a successful non-identical `ImportContentBundle` / authored replacement that keeps that same authored `spawn_group_ref` must **not** resurrect it early as a fresh live full-HP actor
   - the replaced actor must remain dead / non-targetable through the ordinary pending respawn timer, and any add-style visibility presentation during that still-dead window still ends with one trailing `GC DEAD(vid)`
   - identical no-op reimports may continue to short-circuit without mutating lifecycle state; this rule targets non-identical replacements that would otherwise remove and re-register the same authored ref as a new live instance

Current implementation status:
- due-respawn EnterGame / transfer preflight for content-loaded spawn groups is already owned
- still-dead trailing `GC DEAD` replay is owned for both `training_dummy` and content-loaded `spawn_groups` EnterGame / reconnect add-style visibility, including fail-closed target/attack while the dead interval remains open and one-ref/one-actor lookup after that still-dead bootstrap
- same-map return-step / return-home MOVE is live; cross-map return-home remains on delete/readd and now has focused dual-map occupancy coverage (foreign-map delete, home-map add/info/update, one-ref/one-actor, empty foreign-map occupancy, persisted authored home); automatic pending-frame cross-map return-step after `UpdateStaticActor` displace now mirrors that same dual-map anti-leak proof (arms return-step, due flush snaps to authored home via delete/readd with no invented MOVE, clears the pending schedule, restores one-ref/one-actor + empty foreign-map occupancy + persisted authored home); `spawn-leash-bootstrap.md` now freezes that delete/readd path as the Track A bootstrap cross-map return contract (no invented cross-map `MOVE` / `GC WARP`)
- still-dead content-bundle replacement anti-resurrect is now owned: successful non-identical `ImportContentBundle` replacements that keep the same authored `spawn_group_ref` remap pending `HP=0` + absolute respawn deadline onto the newly registered actor before import fanout, so late EnterGame still ends with trailing `GC DEAD` and the actor stays non-targetable through the ordinary timer; engagement / selected-target ownership stay fail-closed across that replacement boundary, while proximity-suppress membership for still-connected subject entity IDs remaps by the same authored `spawn_group_ref` (see the proximity-suppress remapping seam below)
- one-ref/one-actor reconnect / reclaim / fresh EnterGame anti-duplicate coverage is owned beside the existing by-ref fail-closed lookup; session reconnect does not rematerialize a second spawn instance for the same authored `spawn_group_ref`
- same-map live spawn-backed operator/runtime position updates now reuse retained-viewer `MOVE` instead of delete/readd; presentation/name/race refreshes stay on delete/readd (see `spawn-leash-bootstrap.md`)
- EnterGame reclaim chase-deadline cleanup is now owned: when Join reclaims a stale owner that still held practice-mob engagement, pending chase-step deadlines for those released actors are pruned before visibility bootstrap, matching leave/transfer cleanup and preventing a delayed chase MOVE after ownership was dropped
- daemon-restart still-dead spawn-group timer persistence is now owned: the static-actor snapshot carries optional spawn-backed `combat_current_hp=0` plus absolute `respawn_ready_at`, death persists those fields, process restart rematerializes the same authored `spawn_group_ref` as still-dead / non-targetable through the remaining deadline with trailing `GC DEAD` on add-style visibility, due timers still preflight into ordinary live rebuild, and successful respawn clears the still-dead persistence fields; engagement / proximity-suppress / selected-target / chase / return ownership stay fail-closed across restart
- daemon-restart live damaged spawn-group HP persistence is now owned beside that still-dead seam: accepted non-lethal hits persist spawn-backed `combat_current_hp` in `1..max_hp-1` with `respawn_ready_at` omitted, process restart rematerializes the same authored `spawn_group_ref` at that damaged HP / `hp_percent`, full max HP continues to omit the overlay, and engagement / proximity-suppress / selected-target / chase / return ownership stay fail-closed across restart

## First owned daemon-restart still-dead spawn-group timer persistence seam

Question frozen here:

**Once still-dead EnterGame trailing `DEAD`, still-dead content-bundle replacement anti-resurrect, and same-map operator position MOVE already exist, what is the smallest honest persistence contract that keeps a content-loaded spawn-group combatant dead across a `gamed` process restart without inventing engagement remapping or a second spawn scheduler?**

Contract for the first daemon-restart still-dead timer persistence seam:
- while a content-loaded spawn-group combatant is inside its server-owned dead interval (`HP=0` + absolute respawn deadline), a clean `gamed` restart that rematerializes the same authored `spawn_group_ref` from the persisted static-actor snapshot must restore that actor as still-dead rather than as a fresh live full-HP combatant
- the restored actor must remain non-targetable / non-attackable through the remaining absolute deadline, and any add-style visibility presentation after restart (fresh EnterGame, visibility re-entry, retained delete-plus-rebootstrap) must still end with one trailing `GC DEAD(vid)` until that deadline expires
- persistence stores only the still-dead bootstrap facts needed for that rematerialization: `HP=0` plus the absolute respawn ready-at instant keyed by authored `spawn_group_ref` / entity identity already owned by the static-actor snapshot path; it does not invent a second spawn scheduler or a separate dead-timer store family
- once the restored absolute deadline is already due at process start, the ordinary due-respawn EnterGame / pending-frame preflight rebuilds the actor live at authored home before composing static-actor visibility, matching the already-owned due-respawn contract
- engagement / proximity-suppress / selected-target / pending chase / pending return ownership are intentionally **not** restored across process restart; those remain fail-closed session-local ownership that re-arms only after fresh post-restart target / hit / proximity acquisition
- mid-chase / mid-return displacement that is not already persisted as current static-actor position, and non-spawn `training_dummy` daemon-restart durability, remain out of scope for this freeze; live damaged HP above the death floor is owned by the sibling seam below

Current implementation status:
- this seam is now live for content-loaded spawn groups through the static-actor snapshot fields `combat_current_hp` and `respawn_ready_at`
- still-dead EnterGame trailing `DEAD`, still-dead content-bundle replacement anti-resurrect, and same-map operator position MOVE remain owned
- a clean process restart that rematerializes a mid-dead spawn-group actor now restores `HP=0` plus the absolute respawn deadline instead of reconstructing a fresh live full-HP combatant
- malformed still-dead persistence (missing absolute deadline, still-dead fields on non-spawn actors, or combining non-zero HP with a respawn deadline) fails closed at static-actor snapshot validation

Explicit non-goals for this daemon-restart still-dead freeze alone:
- remapping engagement, selected-target, proximity-suppress, chase, or return schedules across restart
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)
- inventing a second spawn scheduler beyond the existing pending-frame flush path
- converting presentation refreshes or respawn rebuild away from delete/readd

## First owned daemon-restart live damaged spawn-group HP persistence seam

Question frozen here:

**Once still-dead spawn-group timer persistence already reuses `combat_current_hp` / `respawn_ready_at` on the static-actor snapshot, what is the smallest honest extension that keeps a content-loaded spawn-group combatant at its live damaged HP across a `gamed` process restart without restoring engagement or inventing a second combat store?**

Contract for the first daemon-restart live damaged HP persistence seam:
- after an accepted non-lethal normal hit against a content-loaded spawn-group combatant, the static-actor snapshot may carry spawn-backed `combat_current_hp` in the open interval `1..max_hp-1` with `respawn_ready_at` omitted
- a clean `gamed` restart that rematerializes the same authored `spawn_group_ref` must restore that damaged HP into the runtime combat map so fresh `TARGET` / attack math continue from the persisted value / matching `hp_percent` instead of silently resetting to full max HP
- full max HP remains the omit form: writers must not persist `combat_current_hp = max_hp`; empty combat overlay means live full HP
- still-dead and live-damaged overlays stay mutually exclusive: non-zero HP may not carry a respawn deadline, and `HP=0` still requires the absolute deadline
- MaxHP for validation resolves through the actor's built-in or portable `combat_profiles` defaults already owned by the snapshot; unknown/unresolved max HP fails closed
- engagement / proximity-suppress / selected-target / pending chase / pending return ownership remain fail-closed across restart and re-arm only after fresh post-restart target / hit / proximity acquisition
- non-spawn standalone `training_dummy` actors and mid-chase / mid-return displacement beyond already-persisted static-actor position remain out of scope; remapping live damaged HP across non-identical content-bundle replacement is frozen by the sibling seam below

Current implementation status:
- accepted non-lethal spawn-group hits persist the damaged `combat_current_hp` overlay through the same static-actor snapshot Save path used by still-dead death
- process restart restores that damaged HP before visibility / combat target selection
- static-actor snapshot validation accepts the live-damaged shape and continues to reject malformed combinations (damaged+deadline, zero HP without deadline, full-HP overlay, non-spawn combat overlays)

Explicit non-goals for this daemon-restart live damaged HP freeze alone:
- remapping engagement, selected-target, proximity-suppress, chase, or return schedules across restart
- remapping live damaged HP across non-identical content-bundle replacement (frozen separately below)
- non-spawn `training_dummy` daemon-restart durability
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)
- inventing a second spawn/combat scheduler beyond the existing pending-frame flush path

## First frozen live damaged spawn-group HP remapping across non-identical content-bundle replacement

Question frozen here:

**Once still-dead content-bundle replacement already remaps `HP=0` + absolute respawn deadline by authored `spawn_group_ref`, and daemon restart already restores live damaged HP for the same ref, what is the smallest honest extension that keeps a mid-fight content-loaded spawn-group combatant at its live damaged HP across a successful non-identical `ImportContentBundle` replacement of that same authored ref without restoring engagement or inventing a second combat store?**

Contract for the first live damaged HP content-bundle replacement remapping seam:
- while a content-loaded spawn-group combatant is live with runtime-owned HP in `1..max_hp-1`, a successful non-identical `ImportContentBundle` / authored replacement that keeps that same authored `spawn_group_ref` must remap that damaged HP onto the newly registered actor before import fanout
- post-replacement `SpawnGroupByRef` / fresh `TARGET` / attack math must continue from that remapped damaged HP / matching `hp_percent` instead of silently resetting to full max HP
- still-dead remapping stays owned and unchanged: `HP=0` + absolute respawn deadline continue to remap through the existing still-dead replacement path
- full max HP remains the omit form and does not invent a damaged overlay during replacement
- engagement / selected-target / pending chase / pending return ownership remain fail-closed across that replacement boundary and re-arm only after fresh post-replacement target / hit / proximity acquisition
- proximity-suppress membership for still-connected subject entity IDs remaps by authored `spawn_group_ref` onto the newly registered actor (see the proximity-suppress remapping seam below); it does not restore engagement or invent selected-target ownership
- identical no-op reimports may continue to short-circuit without mutating lifecycle state; this rule targets non-identical replacements that would otherwise remove and re-register the same authored ref as a fresh live full-HP instance
- non-spawn standalone `training_dummy` actors, remapping engagement across replacement, and mid-chase / mid-return displacement beyond already-persisted static-actor position remain out of scope

Current implementation status:
- still-dead content-bundle replacement anti-resurrect is owned
- daemon-restart live damaged HP persistence is owned
- live damaged HP remapping across non-identical content-bundle replacement is now owned beside that still-dead remapper: successful non-identical same-`spawn_group_ref` imports restore `1..max_hp-1` onto the newly registered actor before import fanout, while engagement / selected-target ownership stay fail-closed
- proximity-suppress remapping across the same non-identical replacement boundary is owned beside that HP remapper

Explicit non-goals for this live damaged replacement remapping freeze alone:
- remapping engagement, selected-target, chase, or return schedules across replacement
- remapping proximity suppress across daemon restart (owned only for live content-bundle replacement below)
- non-spawn `training_dummy` replacement durability
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)
- inventing a second spawn/combat scheduler beyond the existing pending-frame flush path

## First frozen proximity-suppress remapping across non-identical content-bundle replacement

Question frozen here:

**Once leave/re-enter proximity suppress already survives in-radius engagement release, death/respawn seed, death-floor `/restart_here`, and Leave→Join identity changes, what is the smallest honest extension that keeps that same still-inside suppress across a successful non-identical `ImportContentBundle` replacement of the same authored `spawn_group_ref` without restoring engagement or inventing a permanent suppress store?**

Contract for the first proximity-suppress content-bundle remapping seam:
- while a content-loaded spawn-group combatant has proximity-suppress membership for one or more still-connected subject entity IDs, a successful non-identical `ImportContentBundle` / authored replacement that keeps that same authored `spawn_group_ref` must remap those subject IDs onto the newly registered actor before import fanout
- only still-connected subject entity IDs are remapped; disconnected / missing session subjects are dropped rather than inventing a second permanent suppress store
- after remapping, a still-inside suppressed owner must not instantly reacquire through pending-frame proximity acquisition until an explicit leave/re-enter of the actor's effective aggro radius
- engagement / selected-target / pending chase / pending return / delayed-retaliation ownership stay fail-closed across that replacement boundary and re-arm only after fresh post-replacement target / hit / proximity acquisition (after suppress clears)
- still-dead and live-damaged HP remapping stay owned and unchanged beside this suppress remapper
- identical no-op reimports may continue to short-circuit without mutating lifecycle state
- non-spawn standalone `training_dummy` actors remain out of scope; remapping suppress across daemon restart is now owned beside this remapper

Current implementation status:
- `remapSpawnGroupCombatState` now remaps proximity-suppress membership by authored `spawn_group_ref` for still-connected subjects beside still-dead / live-damaged HP remapping
- focused coverage: `TestGameRuntimeProximityAggroSuppressRemapsAcrossContentBundleReplacement`
- daemon-restart proximity-suppress rematerialize is owned separately below

Explicit non-goals for this proximity-suppress remapping freeze alone:
- remapping engagement, selected-target, chase, or return schedules across replacement
- inventing a second permanent suppress store keyed by name/VID beyond the already-owned Leave→Join VID park/claim handoff
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)

## First frozen proximity-suppress rematerialization across daemon restart

Question frozen here:

**Once leave/re-enter proximity suppress already survives in-radius release, death/respawn seed, death-floor `/restart_here`, Leave→Join identity changes, and non-identical same-`spawn_group_ref` content-bundle remapping, what is the smallest honest persistence contract that keeps that same suppress across a clean `gamed` process restart without restoring engagement or inventing a second permanent suppress store?**

Contract for the first daemon-restart proximity-suppress rematerialization seam:

- while a content-loaded spawn-group combatant has proximity-suppress membership for one or more subjects, a clean `gamed` restart that rematerializes the same authored `spawn_group_ref` from the persisted static-actor snapshot must restore that suppress for still-valid character VID park entries
- durable suppress keys are character VID + authored `spawn_group_ref`, not process-local subject/actor entity IDs
- persistence reuses the static-actor snapshot path already owned by still-dead / live-damaged rematerialize: optional per spawn-backed actor `proximity_suppress_vids` (sorted unique character VIDs); omitempty / empty means no suppress overlay
- on `loadPersistedStaticActors`, restore those VIDs into the already-owned `pendingProximityAggroSuppressByVID` park map keyed by the rematerialized actor entity ID so the next `Join` / EnterGame claim path rematerializes suppress onto the new subject entity ID before pending-frame proximity acquisition can re-lock a still-inside owner
- only still-valid character identities are restored; unknown / deleted characters are dropped rather than inventing a second permanent suppress store
- after rematerialize + Join claim, a still-inside suppressed owner must not instantly reacquire through pending-frame proximity acquisition until an explicit leave/re-enter of the actor's effective aggro radius
- engagement / selected-target / pending chase / pending return / delayed-retaliation ownership stay fail-closed across restart and re-arm only after fresh post-restart target / hit / proximity acquisition (after suppress clears)
- still-dead and live-damaged HP persistence stay owned and unchanged beside this suppress rematerializer
- non-spawn standalone `training_dummy` actors remain out of scope

Current implementation status:

- `loadPersistedStaticActors` restores optional `proximity_suppress_vids` into the existing Leave→Join `pendingProximityAggroSuppressByVID` park map for still-valid character VIDs
- writers persist that overlay through the ordinary static-actor snapshot path whenever suppress membership is marked / cleared for spawn-backed actors
- focused coverage: `TestGameRuntimeProximityAggroSuppressRematerializesAcrossDaemonRestart`
- Leave→Join VID park/claim, content-bundle suppress remapping, and still-dead / live-damaged HP daemon-restart persistence remain owned

Explicit non-goals for this daemon-restart proximity-suppress freeze alone:

- remapping engagement, selected-target, chase, or return schedules across restart
- inventing a second permanent suppress store keyed by name beyond the already-owned VID park/claim handoff
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)
- pack AI / synchronized respawn / pathfinding
- non-spawn `training_dummy` suppress durability

Explicit non-goals for this anti-leak freeze alone:
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)
- multi-member spawn packs or pack-wide synchronized respawn
- inventing a second spawn scheduler beyond the existing pending-frame flush path
- remapping engagement or chase/return schedules across non-identical content-bundle replacement (live damaged HP remapping and proximity-suppress remapping across that replacement are frozen above)
- converting generic operator actor presentation updates or respawn rebuild to MOVE

## Explicit non-goals

This slice does **not** yet freeze:
- multi-member spawn packs
- patrol routes or idle roaming
- broader hostile retaliation beyond the first fresh-third-party `TARGET` gate, the first same-target `250ms` normal-attack cadence window, one profile-resolved sustained delayed self-only server-origin retaliation cadence at a time, and the frozen proximity aggro-radius acquisition seam below
- random spawn selection from a pool
- random loot tables, broader kill rewards, or corpse gameplay
- authored interaction metadata on attackable spawn groups
- migrations from old static-actor records into spawn groups

The first owned hostile post-hit reaction is intentionally tiny:
- once a visible content-loaded practice mob from `spawn_groups` accepts its first authoritative hit, fresh third-party `TARGET` attempts now fail closed until the existing death / respawn reset boundary unless the engaged owner's retaliation-driven `0`-HP death clears that engagement first
- that same first authoritative hit also clears any other session's already-selected shared-world combat-target ownership for that same mob and queues one self-only `GC TARGET(0, 0)` clear to each still-live affected third party, so a third party who preselected it before the owner hit cannot keep or visually retain a stale target-selection bypass before later `ATTACK` or fresh `TARGET` retries stay blocked until the owned release boundary is reached
- repeated normal `ATTACK` attempts now also obey one fixed server-owned `250ms` session-local cadence window; denied attempts inside the window stay silent and do not mutate HP or retaliation state, including attempts made immediately after retargeting to another visible practice mob
- while that practice mob stays alive, each accepted owner-side normal hit now also appends one immediate self-only `GC POINT_CHANGE` HP decrement to the engaged player's outgoing success frames
- while that same non-lethal hit leaves the owner alive, the runtime then appends one self `GC DAMAGE_INFO(target_vid, flag = 0, damage = applied_bootstrap_damage)` plus one self `GC DAMAGE_INFO(owner_vid, flag = 0, damage = abs(retaliation_delta))`, and queues both hit-effect frames to currently visible live peers; those peers do not receive the owner's self-only HP target refresh or retaliation point-change
- content-loaded `practice_mob` actors now use the same owned normal-attack HP mutation and owner-side retaliation path as `training_dummy` while their authored-home/current-position leash classification is `at_home` or `within_radius`; if a materialized spawn-backed actor is already `return_required`, fresh target selection and stale selected attacks fail closed with the explicit runtime reason `target_return_required` until an owned respawn, operator return-home, update, or later return/chase executor places it back inside leash; the pure `PlanStaticActorSpawnLeashReturnStep(...)` helper now computes one capped next position toward authored home without mutating runtime state, emitting packets, or claiming autonomous movement; the controlled operator return-home trigger can also restore an exact authored-home position from a `within_radius` drift and clears any selected target/engagement without changing HP or reward metadata
- each accepted normal hit decrements the live runtime HP by the profile's fixed bootstrap damage, clamps at `0`, emits the same deterministic HP-percent result used by the target/point-change slices, and uses the same immediate plus delayed self-only retaliation cadence while the engaged owner stays live
- the first accepted live owner hit also arms one delayed self-only `GC POINT_CHANGE` follow-up beat after `1s`; it arrives through the pending server-frame path even if the owner sends no second `ATTACK`, and non-floor delayed beats also queue the matching owner `GC DAMAGE_INFO(owner_vid, abs(delta))` to currently visible live peers
- while that same engagement remains live, each delayed beat that fires automatically arms the next one after the same fixed delay, so the cadence is now independent from later client attack frames
- that owner-side retaliation point-loss now clamps at the current bootstrap HP floor too: neither the immediate hit-triggered tick nor the delayed follow-up cadence can drive the owner's visible HP below `0`, and once `0` is reached the current slice simply stops further retaliation point-loss without yet claiming broader player-death choreography
- those immediate and delayed owner-side retaliation point-loss beats are currently live-runtime only for that engaged selected session until the bootstrap `0`-HP floor: partial (above-floor) loss does **not** write the persisted account snapshot, and later position-only persistence helpers (`MOVE`, `SYNC_POSITION`, or transfer rebootstrap saves), successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves, plus non-point-bearing slash `/inventory_move` and merchant-buy saves now keep their coordinate, authored use/equip-effect point delta + consumed or carried/equipped item state, carried-slot, or purchase state without overwriting that pre-retaliation point value. Once either retaliation beat reaches the `0`-HP floor, the selected-character bootstrap HP point is persisted as `0` with the owned death/clear frames so a fresh `/phase_select` re-entry, reconnect, or `ENTERGAME` rebuilds from the dead snapshot instead of the pre-retaliation live value; accepted `/restart_here` / `/restart_town` restore race create MaxHP into that persisted snapshot
- when either the immediate retaliation tick or a delayed follow-up beat reaches that owner-side `0`-HP floor, the current slice now emits one self-only `GC POINT_CHANGE(HP, -1, 0)`, then one self-only `GC DEAD(owner_vid)`, then clears the stale engaged target with one self-only `GC TARGET(0, 0)`, and currently visible peers also receive one queued `GC DEAD(owner_vid)` while broader player-death semantics remain out of scope
- when that same owner-side `0`-HP floor is reached while the engaged practice mob still remains alive, the dead owner also stops holding that mob's aggro-lite ownership gate, no further delayed retaliation beat is re-armed for the stale engagement, stale same-target `ATTACK` attempts from the floored owner fail closed, and another visible live session may freshly `TARGET` the same still-live mob without waiting for owner disconnect or mob death / respawn
- once that retaliation floor has already reached `0`, later combat `TARGET` and normal `ATTACK` attempts from that same engaged owner against visible practice mobs also fail closed instead of continuing to reacquire or mutate runtime dummy HP before broader player-death semantics exist
- once that retaliation floor has already reached `0`, later peer-originated exact-name `WHISPER` requests aimed at that same still-connected owner also fail closed before queued target delivery or a synthetic `WHISPER_TYPE_NOT_EXIST` fallback can run
- once that retaliation floor has already reached `0`, later peer-originated `CHAT` requests with types `TALKING`, `PARTY`, `GUILD`, and `SHOUT` still return the live sender's ordinary self echo, but queued peer delivery skips that zero-HP owner recipient entirely under the current bootstrap chat-routing rules
- once that retaliation floor has already reached `0`, later owner-side `MOVE` and `SYNC_POSITION` attempts also fail closed before live position mutation, visibility rebuilds, queued fanout, or coordinate persistence can run
- once that retaliation floor has already reached `0`, later owner-side carried item-drop and carried gold-drop attempts also fail closed before inventory or gold mutation, ground-drop registration, queued visibility frames, or persistence can run
- once that retaliation floor has already reached `0`, later owner-side slash `/inventory_move` attempts also fail closed before carried-slot mutation can run
- once that retaliation floor has already reached `0`, later owner-side slash `/equip_item` and `/unequip_item` attempts also fail closed before carried/equipped item movement, self appearance refresh, or template-backed point mutation can run
- the runtime currently keeps at most one pending delayed follow-up beat at a time for that engaged owner/target pair, so accepted hits while one is already pending do not stack, accelerate, or reset the current delayed-retaliation timer yet
- if the owning live session disappears, clears target intent by movement / sync range loss, crosses an exact-position transfer trigger or warp-interaction rebootstrap boundary, replaces target intent with a fresh `TARGET` on another visible practice mob, an operator return-home trigger resets the engaged spawn actor, or the engaged actor dies / rebuilds before that delay expires, the queued follow-up beat fails closed, current cadence stops, and the abandoned still-live mob's aggro-lite gate is released immediately instead of leaving it orphan-locked forever; a transfer/rebootstrap that had a selected combat target also carries one self-only `GC TARGET(0, 0)` clear in the origin rebuild frames so the client cannot visually retain stale target ownership across maps; at-home return-home is also a reset boundary and advances the combat snapshot version so a pre-return delayed beat cannot fire after fresh reengagement
- if operator/runtime mutation, the controlled return-home trigger, or a successful bundle replacement removes/rebuilds a currently selected live practice mob's runtime snapshot, visible sessions still receive the ordinary static-actor visibility frames when applicable, and any session that still had that mob selected also receives one queued self-only `GC TARGET(0, 0)` so stale target ownership does not survive the runtime-reset boundary
- content-bundle import now suppresses live static-actor visibility fanout while the replacement is in progress, then flushes removed-actor `CHARACTER_DEL` frames followed by imported actors' bootstrap visibility only after the full replacement succeeds; if a later actor/spawn in the same bundle fails and the runtime rolls back, connected sessions do not receive partial `CHARACTER_DEL`, `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`, or `TARGET(0, 0)` clear frames for actors that were never committed, and a pre-existing selected practice mob remains selected and attackable through the restored runtime combat snapshot
- a same-socket `/quit`, `/logout`, or `/phase_select` now counts as that live-session disappearance boundary immediately in the current bootstrap slice, and abrupt session close does too: each path removes the owner from shared-world visibility, cancels any pending delayed follow-up beat, and releases the current aggro-lite target gate before any later disconnect or fresh bootstrap finishes; `/quit` still stays in `GAME` long enough to return its self `CHAT_TYPE_COMMAND quit` delivery, `/logout` continues to transition toward close, `/phase_select` returns to character select while any later bootstrap still requires a fresh `TARGET`, and close tears the session down without a compensating gameplay packet
- that first gate still does **not** imply movement, pathing, pack AI, or a broader aggro system beyond this fixed-delay owner-only cadence

## First owned proximity aggro-radius acquisition seam

Question frozen here:

**Given one live unengaged spawn-backed practice mob that still classifies `at_home` or `within_radius`, and one live same-map player candidate, what is the smallest deterministic server-owned acquisition rule that can establish the already-owned aggro-lite `engaged_by` ownership without requiring an accepted hit first?**

This is the first honest step toward roadmap item “independent mob reaction timing that is not only piggybacked on player hits.” Acquisition itself stays pure. Chase arming and the owned delayed self-only server-origin retaliation cadence are separate session/runtime consumers of the resulting engagement ownership gate; movement/pathfinding still remain later work.

Contract for the first pure helper `EvaluateStaticActorSpawnAggroAcquisition(actor, candidatePosition, radius)` in `internal/worldruntime`:
- fail closed for invalid/non-spawn actors, non-positive aggro `radius`, or invalid candidate positions
- fail closed when the actor currently classifies `return_required`; leash recovery stays owned by the return-step seam and combat targeting already denies `target_return_required`
- fail closed when the actor and candidate are on different maps; no cross-map aggro/warp choreography is owned yet
- succeed only when the candidate is on the same map and Euclidean squared-distance from the actor's current position is `<= radius^2`, using the same bootstrap distance family as leash classification
- by itself the helper never mutates actor position, never updates stores, never sets `engaged_by`, never arms delayed retaliation or chase deadlines, and never queues packets

Bootstrap default radius for the first live consumer:
- `DefaultSpawnAggroRadius = 200`
- deliberately smaller than `DefaultSpawnLeashRadius = 400` so a player can enter aggro without immediately forcing leash/return pressure at the outer leash boundary

First live consumer rules (now implemented on the pending-frame flush path):
- scan from the existing pending-frame / movement-adjacent server path for live spawn-backed practice mobs that currently lack `engaged_by` ownership and still classify `at_home` / `within_radius`
- when exactly one eligible live same-map candidate is inside the default aggro radius, acquire the already-owned aggro-lite engagement ownership for that candidate
- if multiple candidates are inside radius, choose the nearest by Euclidean squared-distance and break ties by ascending player entity ID
- acquisition alone still does **not** invent selected-target ownership and does **not** emit an immediate owner-side retaliation piggyback
- once engagement exists, the session/runtime pending-frame consumer may arm the already-owned delayed self-only server-origin retaliation cadence for that engaged owner using the same fixed `1s` delay and one-pending-beat policy already used after accepted live hits; that delayed cadence still does not invent selected-target ownership
- once engagement exists, the already-owned third-party `target_engaged` gate, chase arming (when present), and release boundaries continue to apply unchanged
- never acquire for dead actors, actors waiting on respawn, zero-HP candidates, or candidates already at the bootstrap HP floor
- the live consumer reuses `EvaluateStaticActorSpawnAggroAcquisition` / `SelectStaticActorSpawnAggroCandidate`, syncs the existing chase-step schedule after a newly established engagement, and lets the engaged owner's session arm delayed retaliation without inventing selected-target ownership or immediate retaliation frames; once that proximity-armed chase deadline becomes due, the pending-frame chase executor applies the ordinary delete/readd step while still inventing no selected combat target
- proximity-only engagement (no selected combat target) must also release when owner `MOVE` / `SYNC_POSITION` leaves `DefaultSpawnAggroRadius`: cancel any pending delayed retaliation beat for that engagement, clear chase schedules for the released actor, mark the ordinary leave/re-enter suppress for that owner, and keep the release silent (no invented self `TARGET(0, 0)` because proximity acquisition never owned selected-target state). Selected-target movement cleanup stays on the existing combat-band / visibility path and is unchanged
- after an explicit engagement release (owner clear-target, proximity leave-radius walk-away, return-home/return-step, operator update, stale-owner cleanup, owner death-floor release, etc.), the same still-inside-radius candidate stays suppressed until it leaves `DefaultSpawnAggroRadius` and re-enters; death and respawn also seed that suppress set for every currently-inside live candidate so a rebuilt or just-killed life does not instantly re-lock nearby players without leave/re-enter
- focused shared-world coverage now proves that in-radius `TARGET(0)` release, death/respawn seed, proximity-armed owner death-floor release followed by same-socket `/restart_here` while still inside radius, and non-identical same-`spawn_group_ref` content-bundle replacement all keep the same nearby owner suppressed through later pending-frame flushes until an explicit leave/re-enter of the actor's effective aggro radius
- subject-side engagement release (`ClearStaticActorCombatEngagementsBySubject`) always marks the releasing subject for proximity suppress even when that subject's shared-world snapshot is already at the bootstrap `0`-HP floor; `seedProximity` still skips floor candidates for bystander seeding, but the releasing owner must remain suppressed across later live-HP recovery (`/restart_here`) without requiring leave/re-enter. Registry coverage: `TestSharedWorldRegistrySubjectReleaseSeedsProximitySuppressWhenOwnerAlreadyAtHPFloor`
- Leave → fresh Join identity changes (`/phase_select` and abrupt disconnect/reconnect) park that subject suppress under character VID on Leave / stale reclaim and rematerialize it onto the new subject entity ID on Join, so a later `/restart_here` while still inside radius stays suppressed the same way; live suppress remains entity-ID keyed (VID is only the handoff key), and actor-side suppress clear also drops pending VID park entries for that actor
- non-identical content-bundle replacement that keeps the same authored `spawn_group_ref` remaps proximity-suppress membership for still-connected subject entity IDs onto the newly registered actor before import fanout (`TestGameRuntimeProximityAggroSuppressRemapsAcrossContentBundleReplacement`); engagement stays fail-closed and daemon-restart suppress rematerialization is owned separately above (`TestGameRuntimeProximityAggroSuppressRematerializesAcrossDaemonRestart`)

Explicit non-goals for this proximity aggro freeze alone:
- immediate owner-side retaliation piggyback without an accepted hit
- inventing selected-target ownership from proximity acquisition alone
- aggro hysteresis / a drop radius distinct from the acquire radius (leave-radius release reuses the actor's effective acquire radius)
- pack aggro, assist calls, or multi-mob linkage
- chase packets, patrol, or pathfinding
- profile-authored per-mob aggro radii beyond the first bootstrap default (owned below through optional `combat_profiles.aggro_radius`)

## First owned profile-authored aggro-radius seam

Question frozen here:

**Once proximity acquisition already uses one bootstrap `DefaultSpawnAggroRadius = 200`, what is the smallest honest authored combat-profile extension that can widen or narrow that acquire radius per registered profile without inventing hysteresis, pack aggro, or a second AI scheduler?**

This is the next Track A follow-on after the owned proximity / chase / leash / anti-leak bootstrap matrix. Cross-map return MOVE / warp choreography remains deferred behind the packet freeze in `spawn-leash-bootstrap.md` and must not be opened as speculative RED.

Contract for optional authored `aggro_radius` on portable `combat_profiles` / `StaticActorCombatProfileDefaults`:
- field name: `aggro_radius` (JSON) / `AggroRadius` (Go)
- type: positive `int32` distance using the same Euclidean squared-distance family as leash / aggro classification
- omitempty / zero means "use bootstrap default": effective radius = `DefaultSpawnAggroRadius` (`200`)
- when present and positive, the effective acquire radius for that registered profile is exactly the authored value
- validation fails closed when `aggro_radius < 0`
- validation fails closed when `aggro_radius > DefaultSpawnLeashRadius` (`400`), so authored acquisition cannot silently stretch past the owned leash / return-required combat gate
- built-in `practice_mob` / `training_dummy` profiles keep effective radius `200` and do not require an authored field
- the pure resolver is `EffectiveStaticActorSpawnAggroRadius(profile)` (or equivalent profile-defaults helper) in `internal/worldruntime`: it returns the effective radius without mutating actor state, engagement, timers, or packets
- live proximity acquisition, leave-radius release, and death/respawn suppress seeding for a spawn-backed actor must reuse that same effective radius for the actor's current combat profile; they must not keep hard-coding `DefaultSpawnAggroRadius` once a profile authors a different value
- leave/re-enter suppress continues to use one radius (no separate drop radius); the suppress boundary is the actor's effective acquire radius
- content-bundle import/export must round-trip a non-default authored `aggro_radius` through `combat_profiles` the same way `respawn_delay_ms` already round-trips

Current implementation status:
- optional authored `aggro_radius` is owned on portable `combat_profiles` / `StaticActorCombatProfileDefaults` and round-trips through content-bundle canonicalize/import/export and file-backed static-actor snapshots
- `EffectiveStaticActorSpawnAggroRadius(profile)` / `EffectiveStaticActorSpawnAggroRadiusForActor(actor)` resolve omitted/zero to `DefaultSpawnAggroRadius` (`200`) and honor positive authored values up to the profile's effective leash radius
- live proximity acquisition, leave-radius release, and death/respawn suppress seeding reuse that effective radius instead of hard-coding the bootstrap default
- negative radii and radii above the effective leash fail closed at registration / bundle / static-snapshot validation
- the checked-in formula and PvE vertical authoring fixtures now author non-default `aggro_radius = 150` on `qa_formula_practice_mob` / `qa_pve_vertical_practice_mob` so manual QA and canonicalize proofs exercise narrowed acquire radius beside formula damage

Explicit non-goals for this profile-authored aggro-radius freeze alone:
- aggro hysteresis / a drop radius distinct from the acquire radius
- pack aggro, assist calls, or multi-mob linkage
- patrol, pathfinding, or target switching
- cross-map aggro / warp choreography
- inventing a second proximity scanner or scheduler beyond the existing pending-frame consumer
- changing the already-owned engagement / chase / retaliation consumers beyond substituting the effective radius

## First owned profile-authored leash-radius seam

Question frozen here:

**Once proximity acquisition already honors optional authored `aggro_radius` and leash / chase / return consumers still hard-code `DefaultSpawnLeashRadius = 400`, what is the smallest honest authored combat-profile extension that can widen or narrow that leash radius per registered profile without inventing pathfinding, hysteresis, or cross-map return MOVE?**

This is the next Track A follow-on after owned profile-authored `aggro_radius`. Cross-map return MOVE / warp choreography remains deferred behind the packet freeze in `spawn-leash-bootstrap.md` and must not be opened as speculative RED.

Contract for optional authored `leash_radius` on portable `combat_profiles` / `StaticActorCombatProfileDefaults`:
- field name: `leash_radius` (JSON) / `LeashRadius` (Go)
- type: positive `int32` distance using the same Euclidean squared-distance family as leash / aggro classification
- omitempty / zero means "use bootstrap default": effective radius = `DefaultSpawnLeashRadius` (`400`)
- when present and positive, the effective leash radius for that registered profile is exactly the authored value
- validation fails closed when `leash_radius < 0`
- validation fails closed when a positive authored `leash_radius` is strictly less than the profile's effective aggro radius (`authored aggro_radius` when positive, otherwise `DefaultSpawnAggroRadius`), so authored acquisition cannot silently stretch past the owned leash / return-required combat gate
- once this seam is live, authored `aggro_radius` validation tightens the same way: a positive authored aggro must not exceed the profile's effective leash radius (authored `leash_radius` when positive, otherwise `DefaultSpawnLeashRadius`)
- built-in `practice_mob` / `training_dummy` profiles keep effective leash radius `400` and do not require an authored field
- the pure resolver is `EffectiveStaticActorSpawnLeashRadius(profile)` (plus `EffectiveStaticActorSpawnLeashRadiusForActor(actor)`) in `internal/worldruntime`: it returns the effective radius without mutating actor state, engagement, timers, or packets
- live leash classification, return-step / return-home planning and execution, chase-step planning and execution, and `target_return_required` gating for a spawn-backed actor must reuse that same effective radius for the actor's current combat profile; they must not keep hard-coding `DefaultSpawnLeashRadius` once a profile authors a different value
- read-only operator leash endpoints may still accept an explicit query `radius` override for inspection; omitting that override should resolve through the actor's effective leash radius rather than inventing a second hard-coded default
- content-bundle import/export must round-trip a non-default authored `leash_radius` through `combat_profiles` the same way `aggro_radius` / `respawn_delay_ms` already round-trip

Current implementation status:
- optional authored `leash_radius` is owned on portable `combat_profiles` / `StaticActorCombatProfileDefaults` and round-trips through content-bundle canonicalize/import/export and file-backed static-actor snapshots
- `EffectiveStaticActorSpawnLeashRadius(profile)` / `EffectiveStaticActorSpawnLeashRadiusForActor(actor)` resolve omitted/zero to `DefaultSpawnLeashRadius` (`400`) and honor positive authored values that stay at or above the profile's effective aggro radius
- live leash classification, return-step / return-home, chase-step planning/execution, and `target_return_required` gating reuse that effective radius instead of hard-coding the bootstrap default
- read-only operator leash GET endpoints keep an explicit query `radius` override and default omitted lookups through the actor's effective leash radius
- negative leash radii and positive leash radii below the profile's effective aggro fail closed; positive authored aggro must also stay within the profile's effective leash
- unengaged `within_radius` homeward recovery after chase/engagement release is now owned beside return-step: `PlanStaticActorSpawnLeashHomewardStep` plus the pending-frame homeward executor step the actor toward authored home with same-map retained-viewer `MOVE` while combat remains allowed and engagement stays cleared; `return_required` recovery and operator exact-home `return-home` are unchanged; operator/runtime same-map position `UpdateStaticActor` that leaves a live unengaged spawn-backed actor `within_radius` now re-arms that same pending homeward deadline
- the checked-in formula and PvE vertical authoring fixtures now author non-default `leash_radius = 350` beside `aggro_radius = 150` so omitted-radius leash GET and proximity smoke can prove profile-effective radii without relying only on bootstrap `400`

Explicit non-goals for this profile-authored leash-radius freeze alone:
- pathfinding, navmesh, patrol, or continuous interpolation
- aggro hysteresis / a drop radius distinct from the acquire radius
- pack aggro, assist calls, or multi-mob linkage
- inventing cross-map return MOVE / `GC WARP` choreography (frozen as delete/readd / direct-home rebuild in `spawn-leash-bootstrap.md`)
- inventing a second return/chase scheduler beyond the existing pending-frame consumers
- changing the already-owned engagement / chase / retaliation consumers beyond substituting the effective leash radius
- remapping engagement across non-identical content-bundle replacement (live damaged HP remapping and proximity-suppress remapping across that replacement are frozen above; live damaged HP across clean daemon restart remains owned; proximity-suppress rematerialization across daemon restart is owned above)

## Success definition

After this document lands, the repository should be able to say:
- there is now one project-owned authored content seam for attackable non-player spawns: `spawn_groups`
- the first spawn group is intentionally size `1`, stationary, and combat-profile driven
- authored content now has a stable way to say which combatant should exist, where it should appear, which `combat_profile` it should use, and which deterministic EXP/gold/drop descriptor should apply on its killing hit
- respawn ownership is no longer implied to come from ad hoc runtime registration; it is conceptually anchored to the authored spawn-group `ref`
- one content-authored practice mob can now be imported through `spawn_groups` with bundle-replacement bootstrap visibility delayed until the replacement is fully committed, fight using the owned built-in `training_dummy` or `practice_mob` combat profile, rebuild after death through the existing server-driven respawn loop, reject fresh third-party `TARGET` attempts after its first accepted hit while that engaged owner still lives, proactively clear any stale preselected third-party target with one self-only `GC TARGET(0, 0)` when that same first hit establishes or preserves engagement, release that same gate again if retaliation kills the owner before the mob dies, apply one fixed same-target `250ms` normal-attack cadence gate, one immediate self-only owner HP decrement per accepted live hit, one sustained delayed self-only server-origin follow-up cadence at a time, partial (above-floor) retaliation point-loss that stays runtime-only across fresh `/phase_select` re-entry or reconnect and also stays out of later position-only `MOVE` / `SYNC_POSITION` / transfer rebootstrap saves, while the bootstrap `0`-HP floor itself persists with the owned death/clear frames, successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves, plus non-point-bearing slash `/inventory_move` and merchant-buy saves while those helpers still persist coordinates, authored use/equip-effect point delta + consumed or carried/equipped item state, carried-slot state, or purchased item/gold state, self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` and one visible-peer `GC DEAD(owner_vid)` fanout when that retaliation floor reaches `0` HP, treat same-socket `/quit`, `/logout`, and `/phase_select` plus abrupt session close as immediate owner-disappearance boundaries for queued delayed retaliation + aggro release, also release the abandoned still-live mob immediately when movement / sync clears target intent or a fresh `TARGET` retargets another visible practice mob, close an already-open merchant window there with one self-only `GC::SHOP END`, and fail-closed owner-side combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner carried gold-drop attempts, owner slash `/use_item` and carried-slot `ITEM_USE`, `/inventory_move`, `/equip_item`, and `/unequip_item` attempts, owner pee...
