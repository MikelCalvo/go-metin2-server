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

The repository-owned example bundle at `docs/examples/bootstrap-npc-service-bundle.json` now includes one `spawn_groups` practice mob with a deliberately non-zero bootstrap reward descriptor. That example is intended for local QA of the owned target -> hit -> death -> reward-drop loop; broader loot tables and quest/corpse reward systems remain out of scope.

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
  - portable `combat_profiles[].profile` identities must already be canonical lowercase snake-case on input; canonicalization and runtime import reject surrounding whitespace, uppercase letters, dots, hyphens, leading digits, blank names, and built-in profile names instead of trimming or rewriting them into a different process-local key
  - if later bundle canonicalization, validation, or static-actor replacement fails after registering new combat profiles, the bootstrap importer/canonicalizer rolls back the profile registrations it introduced for that failed import; already-registered local profiles are left untouched
  - if a bundle carries duplicate `combat_profiles` snapshots for the same profile name, even when the duplicate snapshots are identical, the import fails closed before registering that profile or materializing spawn actors so hand-authored portable bundles cannot race two conflicting/default definitions through the runtime seam
  - portable `combat_profiles` snapshots are validated against the same identity and damage-shape rules as runtime profile registration: explicit `damage_per_normal_attack`, when present, must match the deterministic `max(1, attack_value - defense_value)` formula, and formula damage above `max_hp` fails closed instead of being silently clamped during bundle import
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
  - bundle summaries now expose each spawn group's authored `x`, `y`, and `race_num` placement/template identity alongside its `reward_drop_vnums` list and deterministic `reward_drop_items` list, so operators can inspect spawn placement, visual template, resolved item names, stackability, max counts, optional shop buy/sell prices, owned transfer/merchant rejection metadata, and direct-use guard metadata (`confirm_when_use`, `quest_use`, `quest_use_multiple`, `applicable`, and `use_reject_message`) without cross-reading the full bundle sections
  - bundle summaries also expose aggregate reward totals (`reward_experience_total`, `reward_gold_total`, `reward_drop_item_count`) and deterministic `reward_drops` grouped by item vnum with `source_count`, resolved item name, stackability, max count, optional shop buy/sell prices, owned transfer/merchant rejection metadata, and direct-use guard metadata; this gives operators a compact reward audit before importing a candidate bundle
  - each per-map summary row now carries the reward totals contributed by spawn groups on that map (`reward_experience_total`, `reward_gold_total`, `reward_drop_item_count`) alongside static/interactable/spawn occupancy, so an operator can spot overloaded reward zones without expanding the full `spawn_groups` payload
  - import previews now carry signed top-level and per-map reward amount deltas for `reward_experience_total` and `reward_gold_total`, alongside the existing count deltas, so dry-run replacement output shows reward-budget increases and decreases before an import mutates live authored content
  - import previews now also carry exact per-map `static_actors` and `spawn_groups` delta rows under `deltas.maps[]`, so an operator can inspect which authored actors and spawn identities are added, removed, or changed on each affected map without manually correlating top-level rows with coordinates
  - import previews now also carry exact `deltas.combat_profiles` rows for portable custom combat-profile snapshots that are added, removed, or changed, so an operator can see HP/damage/formula/presentation/respawn/reward default changes before a replacement bundle is applied
  - `GET /local/content-bundle/combat-profiles/{profile}` is the loopback-only exact-profile reader over the same live exported bundle summary; it accepts only canonical lowercase snake-case profile names, returns one portable custom `combat_profiles[]` snapshot when present, returns `404` when the live exported bundle does not carry that profile, and exists for local QA/introspection rather than gameplay protocol or content mutation
  - non-zero values use the narrow reward contract in `non-player-reward-bootstrap.md` on the accepted killing hit
  - reward data belongs to the authored spawn group and round-trips through content bundles, static-actor snapshots, and runtime import/export; it is not live character persistence by itself
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

### Combat profile owns
- combat defaults and rules shared by authored actors using that profile
- for the current bootstrap profiles, that includes the existing training-dummy HP/death/respawn semantics already frozen elsewhere
- the current profile-default seam is deliberately compact: `max_hp`, `damage_per_normal_attack`, `attack_value`, `defense_value`, descriptor-only `level`, descriptor-only `rank`, `respawn_delay`, `retaliation_point_delta`, and the reward descriptor documented in `non-player-reward-bootstrap.md`
- `attack_value` / `defense_value` are now profile-owned authored stat defaults used by the first deterministic registered-profile damage formula (`max(1, attack_value - defense_value)`); `damage_per_normal_attack` remains the legacy fallback used to preserve older bootstrap profile behavior, legacy-damage profiles that omit `attack_value` canonicalize it as `damage_per_normal_attack + defense_value`, formula-first profiles that omit `damage_per_normal_attack` now canonicalize that legacy fallback from the same attack/defense formula during registration, and profiles whose explicit formula damage would exceed `max_hp` fail closed instead of being silently capped
- `level` / `rank` are now profile-owned metadata for later mob presentation, reward, or formula slices: built-in `training_dummy` and `practice_mob` default to `level = 1` and `rank = 0`, registered profiles preserve explicit values, omitted registered-profile `level` canonicalizes to the same bootstrap level `1`, and omitted registered-profile `rank` remains `0`
- runtime static-actor snapshots now expose the resolved profile presentation metadata as `combat_level` and `combat_rank`, plus a non-default `retaliation_point_delta` when the resolved profile uses a custom negative owner-retaliation amount, so loopback introspection, map/visibility snapshots, and later presentation/hostility slices can inspect the effective defaults without re-resolving the profile name; imported spawn actors and later generic static-actor updates preserve those effective fields while keeping the same `spawn_group_ref` / `combat_profile`; current HP mutation, reward payout, target carriers, and respawn timing still do not read the presentation fields, and an omitted `retaliation_point_delta` in these runtime snapshots means the bootstrap default `-1` point-loss applies
- runtime code now has a narrow registration seam for additional bootstrap combat profiles with those same defaults, so later authored profiles can be introduced without hard-coding every new name into target/attack/respawn validation
- registered profile defaults are used by the same shared-world target/attack/death/respawn loop as built-in profiles: target selection starts from the registered `max_hp`, accepted normal attacks apply the registered attack/defense formula, HP percent is derived from that registered max, spawn-backed deaths can resolve the registered profile's reward descriptor, the dead timer uses the registered `respawn_delay`, and the rebuild restores the actor to the registered full HP
- registered profile defaults now also own the first optional deterministic owner-side retaliation amount for spawn-backed practice mobs: omitted or `0` `retaliation_point_delta` canonicalizes to the current bootstrap `-1` HP decrement, negative values are preserved for both the immediate hit-triggered tick and delayed server-origin cadence, and positive values fail closed because this bootstrap retaliation seam cannot heal or buff the owner
- when an existing static/spawn actor is explicitly updated to a different combat profile while its old runtime combat instance is already dead, the update cancels the old pending respawn timer, clears the dead HP state, and makes the updated actor immediately targetable as a fresh live snapshot at the new profile's full HP; same-profile presentation or placement edits keep using the ordinary update/target-clear rules without inventing a new respawn timer
- registered profile names also use the same first aggro-lite ownership and retaliation gate as built-in spawn-backed practice mobs: once the first owner lands an accepted hit in the current live loop, fresh third-party `TARGET` attempts fail closed until the existing engagement reset boundaries release or rebuild that actor, and accepted live owner hits plus the delayed server-origin cadence emit the same self-only `GC POINT_CHANGE` HP decrements as the built-in bootstrap practice profiles; an accepted non-zero retarget by the engaged owner cancels the abandoned target's session-local pending delayed beat but does not release that current-life engagement to third-party target selection, while an explicit client `TARGET(0)`, owner disappearance/rebootstrap, owner zero-HP stale cleanup, actor update/removal, or mob death/respawn still releases or rebuilds the ownership boundary; a recorded owner that has already reached the bootstrap `0`-HP floor is treated as stale engagement ownership on the next fresh third-party `TARGET` attempt, so a still-live mob can be reacquired without waiting for its own death / respawn cycle
- registered profile names are immutable for the lifetime of the current process: registration fails closed when the name is blank, has non-canonical surrounding whitespace, is not a lowercase ASCII snake-case identifier (`[a-z][a-z0-9_]*`), names a built-in bootstrap profile, already exists, has neither a legacy `damage_per_normal_attack` value nor an explicit formula `attack_value`, supplies both legacy damage and explicit formula values that disagree after canonicalization, has invalid HP/formula/respawn defaults after canonicalization, has `respawn_delay_ms` outside the positive range that can round-trip safely to the runtime `time.Duration` respawn timer, has effective `damage_per_normal_attack > max_hp`, has explicit formula damage greater than `max_hp`, carries a positive `retaliation_point_delta`, or carries an invalid reward descriptor
- `gamed` exposes a loopback-only operator profile endpoint for process-local profile authoring and inspection:
  - `GET /local/static-actor-combat-profiles`
  - returns the deterministic sorted list of built-in and registered combat-profile defaults, including derived `damage_per_normal_attack`, formula stats, presentation metadata, `respawn_delay_ms`, non-default `retaliation_point_delta`, and cloned/sorted reward descriptors using stable snake-case `death_reward` JSON keys (`experience`, `gold`, `drop_vnums`)
  - `POST /local/static-actor-combat-profiles`
  - JSON fields: `profile`, `max_hp`, optional `damage_per_normal_attack`, optional formula fields `attack_value` / `defense_value`, optional presentation `level` / `rank`, `respawn_delay_ms`, optional `retaliation_point_delta`, and optional `death_reward` with `experience`, `gold`, and `drop_vnums`
  - success returns the canonicalized profile defaults, including derived `damage_per_normal_attack`, effective `retaliation_point_delta`, and sorted/deduplicated reward drops
  - request bodies are bounded to 4 KiB; oversized bodies return `413`, and invalid UTF-8 is rejected before JSON decoding or profile registration
  - content-bundle canonicalization now snapshots custom registered profiles referenced by `spawn_groups` and `static_actors` in the top-level `combat_profiles` array, including formula stats, presentation metadata, respawn delay, retaliation point delta when it differs from the bootstrap default, and death-reward defaults, so exported authored combat content is self-describing for local QA instead of depending only on process-local profile registration state
  - built-in profile names are intentionally omitted from `combat_profiles` because their defaults are runtime-owned bootstrap constants, while custom profiles used by either authored collection are deduplicated and sorted by profile name
  - invalid JSON, unknown fields, non-loopback callers, built-in/duplicate/invalid profile names, profile names with surrounding whitespace, invalid formula defaults, invalid respawn delay, and invalid reward descriptors fail closed without registration
- that registration seam is still process-local operator tooling; content-bundle import/export now carries deterministic `combat_profiles` snapshots for custom authored profiles, but runtime still rejects malformed profile definitions and never canonicalizes padded profile names into a different key
- content-bundle import compares custom `combat_profiles` against existing process-local profile defaults after applying the same canonical formula/default expansion used at registration time, so a formula-first profile that omits `damage_per_normal_attack`, `level`, or `retaliation_point_delta` can be reimported idempotently while conflicting definitions still fail closed
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
- for the shipped file-backed `gamed` runtime, the authored-home respawn position is written back to the static-actor snapshot before the live actor is revived; a failed write leaves the dead/displaced actor and due respawn timer intact for retry instead of partially reviving runtime state or queuing visibility frames
- same-profile runtime/operator edits made while a spawn-backed actor is dead may move the materialized snapshot for inspection, but they do not convert that dead return-required actor into an automatic return-step candidate; the pending respawn timer stays the only server-owned lifecycle action until it rebuilds the actor at authored home

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

Import should reject malformed spawn groups before mutating live runtime state. The bundle canonicalization path now keeps spawn-group names explicit instead of synthesizing them from `ref`, rejects duplicate or non-canonical `ref` values without trimming the authored identifier into a different key, and preserves the prior authored/runtime snapshot when validation fails.

Runtime static-actor snapshots are also part of this contract because export, persistence rollback, map/visibility introspection, and respawn/rebuild code all round-trip through the same snapshot shape. A materialized spawn-group actor must therefore preserve its authored `spawn_group_ref` and normalized `combat_profile` in the live runtime snapshot, not just in the initial content-bundle record or file-backed store.

## Content bundle operator/runtime boundary

The bootstrap content-bundle surface uses the same top-level `spawn_groups` collection for export and import through the local operator bundle endpoint.

Current runtime rules:
- spawn-backed live actors export as `spawn_groups`, not as ordinary `static_actors`
- exported spawn-group `map_index`, `x`, and `y` come from the preserved authored spawn home when present, not from a displaced materialized current position; older snapshots without `spawn_home` fall back to their current actor position for compatibility
- importing a bundle with `spawn_groups` materializes one runtime static actor per group with the authored `spawn_group_ref`
- the imported actor uses the authored placement, `race_num`, and normalized `combat_profile`
- if the candidate bundle canonicalizes to the exact same content bundle currently exported by the runtime, import is treated as a no-op: the runtime returns the canonical bundle without rewriting item templates, interaction definitions, static actors, live combat HP, selected-target ownership, pending respawn/retaliation state, or queued visibility fanout
- when a successful import materializes a spawn-backed actor while players are already online, the runtime enqueues the normal static-actor visibility bootstrap burst (`CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`) only to sessions that currently share the actor's visible world/AOI
- when a successful import replaces a previously visible static actor, the runtime first enqueues the old actor's `CHARACTER_DEL` and then enqueues newly imported actor bootstrap bursts to sessions that currently share those actors' visible world/AOI
- sessions outside the configured visibility policy do not receive the imported spawn actor until a later enter-game or AOI/transfer visibility rebuild makes it visible
- before mutating the runtime replacement set, import must be able to export the current canonical content bundle used for rollback; if that preflight export fails after a bundle temporarily registered process-local combat profiles, those imported profiles are rolled back before the error is reported
- bundle canonicalization now rejects a portable `combat_profiles` snapshot when its `profile` name already resolves to a registered process-local profile with different canonical defaults; matching snapshots remain idempotent, but conflicting snapshots fail closed before runtime import can mutate content or profile state
- if static-actor persistence fails after interaction definitions have already been replaced, import fails closed and restores the previously exported content bundle before reporting failure; online sessions do not receive staged delete/add visibility frames for content that failed to commit
- if bundle replacement fails after removing a previously selected spawn-backed actor, rollback restores the prior actor plus its runtime combat snapshot, selected-target ownership, HP, respawn/retaliation metadata, and reward descriptor state before reporting failure; the waiting session must not receive a staged `CHARACTER_DEL`, imported actor burst, or `TARGET(0, 0)` clear for content that never committed
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
`POST /local/spawn-groups/{entity_id}/return-home` is the paired loopback-only controlled exact-home return trigger. It accepts no request body, restores one live spawn-backed actor to preserved authored home, fails closed for missing/non-spawn/dead actors or failed static snapshot persistence, preserves HP/death/reward/combat metadata, releases current engagement/selected-target ownership for that actor, clears any pending automatic return-step deadline for that actor, and reuses ordinary static-actor visibility deltas so old-position viewers receive `CHARACTER_DEL`, home-position viewers receive the normal add/info/update burst, and retained viewers receive delete-plus-readd at home before target-clear frames. Removing a materialized spawn-backed actor also clears any pending automatic return-step deadline for that entity ID after the removal commits.
`GET /local/maps/{map_index}/static-actors` returns the deterministic full static-actor subset for one effective map, including both ordinary service actors and spawn-backed actors, without requiring callers to fetch the full `/local/maps` occupancy row or filter the global `/local/static-actors` list.
`GET /local/maps/{map_index}/spawn-groups` returns the deterministic spawn-backed subset for one effective map without requiring callers to fetch the full `/local/maps` occupancy row or filter the global `/local/spawn-groups` list.
`GET /local/maps/{map_index}/spawn-group-leashes?radius=<positive-int>` returns the deterministic spawn-backed subset plus current leash classification for one effective map without requiring callers to fan out exact entity leash queries themselves.
`GET /local/maps/{map_index}/static-actor-respawns` returns the pending server-driven respawn timers whose dead actor currently belongs to one effective map, using the same `entity_id`, `ready_at`, `remaining_ms`, and dead static-actor row shape as `/local/static-actor-respawns`.
`GET /local/maps/{map_index}/combat-targets` returns the active selected combat-target snapshots whose selected subject currently belongs to one effective map, using the same subject/target/HP/engagement row shape as `/local/combat-targets`.
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
- resolved descriptor metadata (`combat_level`, `combat_rank`)
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

## Explicit non-goals

This slice does **not** yet freeze:
- multi-member spawn packs
- patrol routes or idle roaming
- broader hostile retaliation or aggro-lite behavior beyond the first fresh-third-party `TARGET` gate, the first same-target `250ms` normal-attack cadence window, plus one profile-resolved sustained delayed self-only server-origin retaliation cadence at a time
- random spawn selection from a pool
- random loot tables, broader kill rewards, or corpse gameplay
- authored interaction metadata on attackable spawn groups
- migrations from old static-actor records into spawn groups

The first owned hostile post-hit reaction is intentionally tiny:
- once a visible content-loaded practice mob from `spawn_groups` accepts its first authoritative hit, fresh third-party `TARGET` attempts now fail closed until the existing death / respawn reset boundary unless the engaged owner's retaliation-driven `0`-HP death clears that engagement first
- that same first authoritative hit also clears any other session's already-selected shared-world combat-target ownership for that same mob and queues one self-only `GC TARGET(0, 0)` clear to each still-live affected third party, so a third party who preselected it before the owner hit cannot keep or visually retain a stale target-selection bypass before later `ATTACK` or fresh `TARGET` retries stay blocked until the owned release boundary is reached
- repeated normal `ATTACK` attempts now also obey one fixed server-owned `250ms` session-local cadence window; denied attempts inside the window stay silent and do not mutate HP or retaliation state, including attempts made immediately after retargeting to another visible practice mob
- while that practice mob stays alive, each accepted owner-side normal hit now also appends one immediate self-only `GC POINT_CHANGE` HP decrement to the engaged player's outgoing success frames
- while that same non-lethal hit leaves the owner alive, the runtime then appends one self `GC DAMAGE_INFO(target_vid, flag = 0, damage = applied_bootstrap_damage)` and queues the same hit-effect frame to currently visible live peers; those peers do not receive the owner's self-only HP target refresh or retaliation point-change
- content-loaded `practice_mob` actors now use the same owned normal-attack HP mutation and owner-side retaliation path as `training_dummy` while their authored-home/current-position leash classification is `at_home` or `within_radius`; if a materialized spawn-backed actor is already `return_required`, fresh target selection and stale selected attacks fail closed with the explicit runtime reason `target_return_required` until an owned respawn, operator return-home, update, or later return/chase executor places it back inside leash; the pure `PlanStaticActorSpawnLeashReturnStep(...)` helper now computes one capped next position toward authored home without mutating runtime state, emitting packets, or claiming autonomous movement; the controlled operator return-home trigger can also restore an exact authored-home position from a `within_radius` drift and clears any selected target/engagement without changing HP or reward metadata
- each accepted normal hit decrements the live runtime HP by the profile's fixed bootstrap damage, clamps at `0`, emits the same deterministic HP-percent result used by the target/point-change slices, and uses the same immediate plus delayed self-only retaliation cadence while the engaged owner stays live
- the first accepted live owner hit also arms one delayed self-only `GC POINT_CHANGE` follow-up beat after `1s`; it arrives through the pending server-frame path even if the owner sends no second `ATTACK`
- while that same engagement remains live, each delayed beat that fires automatically arms the next one after the same fixed delay, so the cadence is now independent from later client attack frames
- that owner-side retaliation point-loss now clamps at the current bootstrap HP floor too: neither the immediate hit-triggered tick nor the delayed follow-up cadence can drive the owner's visible HP below `0`, and once `0` is reached the current slice simply stops further retaliation point-loss without yet claiming broader player-death choreography
- those immediate and delayed owner-side retaliation point-loss beats are currently live-runtime only for that engaged selected session: they do **not** write the persisted account snapshot, and later position-only persistence helpers (`MOVE`, `SYNC_POSITION`, or transfer rebootstrap saves), successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves, plus non-point-bearing slash `/inventory_move` and merchant-buy saves now keep their coordinate, authored use/equip-effect point delta + consumed or carried/equipped item state, carried-slot, or purchase state without overwriting that pre-retaliation point value, so a fresh `/phase_select` re-entry or reconnect rebuilds from the pre-retaliation point value plus any later owned use/equip delta until broader player-death persistence or respawn semantics are owned
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
- if the owning live session disappears, clears target intent by movement / sync range loss, replaces target intent with a fresh `TARGET` on another visible practice mob, an operator return-home trigger resets the engaged spawn actor, or the engaged actor dies / rebuilds before that delay expires, the queued follow-up beat fails closed, current cadence stops, and the abandoned still-live mob's aggro-lite gate is released immediately instead of leaving it orphan-locked forever; at-home return-home is also a reset boundary and advances the combat snapshot version so a pre-return delayed beat cannot fire after fresh reengagement
- if operator/runtime mutation, the controlled return-home trigger, or a successful bundle replacement removes/rebuilds a currently selected live practice mob's runtime snapshot, visible sessions still receive the ordinary static-actor visibility frames when applicable, and any session that still had that mob selected also receives one queued self-only `GC TARGET(0, 0)` so stale target ownership does not survive the runtime-reset boundary
- content-bundle import now suppresses live static-actor visibility fanout while the replacement is in progress, then flushes removed-actor `CHARACTER_DEL` frames followed by imported actors' bootstrap visibility only after the full replacement succeeds; if a later actor/spawn in the same bundle fails and the runtime rolls back, connected sessions do not receive partial `CHARACTER_DEL`, `CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`, or `TARGET(0, 0)` clear frames for actors that were never committed, and a pre-existing selected practice mob remains selected and attackable through the restored runtime combat snapshot
- a same-socket `/quit`, `/logout`, or `/phase_select` now counts as that live-session disappearance boundary immediately in the current bootstrap slice, and abrupt session close does too: each path removes the owner from shared-world visibility, cancels any pending delayed follow-up beat, and releases the current aggro-lite target gate before any later disconnect or fresh bootstrap finishes; `/quit` still stays in `GAME` long enough to return its self `CHAT_TYPE_COMMAND quit` delivery, `/logout` continues to transition toward close, `/phase_select` returns to character select while any later bootstrap still requires a fresh `TARGET`, and close tears the session down without a compensating gameplay packet
- that first gate still does **not** imply movement, pathing, pack AI, or a broader aggro system beyond this fixed-delay owner-only cadence

## Success definition

After this document lands, the repository should be able to say:
- there is now one project-owned authored content seam for attackable non-player spawns: `spawn_groups`
- the first spawn group is intentionally size `1`, stationary, and combat-profile driven
- authored content now has a stable way to say which combatant should exist, where it should appear, which `combat_profile` it should use, and which deterministic EXP/gold/drop descriptor should apply on its killing hit
- respawn ownership is no longer implied to come from ad hoc runtime registration; it is conceptually anchored to the authored spawn-group `ref`
- one content-authored practice mob can now be imported through `spawn_groups` with bundle-replacement bootstrap visibility delayed until the replacement is fully committed, fight using the owned built-in `training_dummy` or `practice_mob` combat profile, rebuild after death through the existing server-driven respawn loop, reject fresh third-party `TARGET` attempts after its first accepted hit while that engaged owner still lives, proactively clear any stale preselected third-party target with one self-only `GC TARGET(0, 0)` when that same first hit establishes or preserves engagement, release that same gate again if retaliation kills the owner before the mob dies, apply one fixed same-target `250ms` normal-attack cadence gate, one immediate self-only owner HP decrement per accepted live hit, one sustained delayed self-only server-origin follow-up cadence at a time, runtime-only retaliation point-loss that does not yet persist across fresh `/phase_select` re-entry or reconnect and now also stays out of later position-only `MOVE` / `SYNC_POSITION` / transfer rebootstrap saves, successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves, plus non-point-bearing slash `/inventory_move` and merchant-buy saves while those helpers still persist coordinates, authored use/equip-effect point delta + consumed or carried/equipped item state, carried-slot state, or purchased item/gold state, self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` and one visible-peer `GC DEAD(owner_vid)` fanout when that retaliation floor reaches `0` HP, treat same-socket `/quit`, `/logout`, and `/phase_select` plus abrupt session close as immediate owner-disappearance boundaries for queued delayed retaliation + aggro release, also release the abandoned still-live mob immediately when movement / sync clears target intent or a fresh `TARGET` retargets another visible practice mob, close an already-open merchant window there with one self-only `GC::SHOP END`, and fail-closed owner-side combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner carried gold-drop attempts, owner slash `/use_item` and carried-slot `ITEM_USE`, `/inventory_move`, `/equip_item`, and `/unequip_item` attempts, owner pee...
