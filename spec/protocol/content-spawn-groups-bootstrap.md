# Content Spawn-Groups Bootstrap

This document freezes the first authored content contract for attackable non-player spawns in `go-metin2-server`.

It sits on top of:
- `combat-training-dummy-bootstrap.md`
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
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
- deterministic content import/export and validation before runtime mutation

This contract does **not** yet claim:
- roaming/wandering/pathing AI
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
  - bundles may include a matching top-level `combat_profiles` snapshot for non-built-in profile names; runtime import registers those profile defaults before validating/importing `spawn_groups`, so portable bundles can carry their authored combat profile and reward defaults without requiring a prior local profile registration step
  - if later bundle validation or static-actor replacement fails after registering new combat profiles, the bootstrap importer rolls back the profile registrations it introduced for that failed import; already-registered local profiles are left untouched
  - if a bundle carries a `combat_profiles` snapshot for a profile name that is already registered locally, the snapshot must exactly match the registered canonical defaults; conflicting snapshots fail closed before spawn actors are materialized so portable bundle imports cannot silently reinterpret authored combatants with different HP/damage/reward defaults
- `reward_experience`, `reward_gold`, `reward_drop_vnums`
  - optional authored death-reward descriptor fields
  - if all reward fields are omitted or zero/empty, bundle canonicalization now applies the selected combat profile's bootstrap death-reward defaults; the built-in `practice_mob` and `training_dummy` profiles remain rewardless, while registered reward-bearing profiles can provide deterministic defaults
  - explicit non-zero reward fields override profile defaults for that spawn group
  - non-empty drop-vnum lists canonicalize into ascending deterministic order across content bundles and file-backed static-actor snapshots
  - non-zero values use the narrow reward contract in `non-player-reward-bootstrap.md` on the accepted killing hit
  - reward data belongs to the authored spawn group and round-trips through content bundles, static-actor snapshots, and runtime import/export; it is not live character persistence by itself
- operator/runtime edits that preserve the same `spawn_group_ref` must preserve the authored `combat_profile` and reward descriptor while changing mutable actor presentation/placement fields; delete/recreate or bundle replacement remains the explicit way to replace reward metadata
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
- the current profile-default seam is deliberately compact: `max_hp`, `damage_per_normal_attack`, `attack_value`, `defense_value`, descriptor-only `level`, descriptor-only `rank`, `respawn_delay`, and the reward descriptor documented in `non-player-reward-bootstrap.md`
- `attack_value` / `defense_value` are now profile-owned authored stat defaults used by the first deterministic registered-profile damage formula (`max(1, attack_value - defense_value)`); `damage_per_normal_attack` remains the legacy fallback used to preserve older bootstrap profile behavior, legacy-damage profiles that omit `attack_value` canonicalize it as `damage_per_normal_attack + defense_value`, formula-first profiles that omit `damage_per_normal_attack` now canonicalize that legacy fallback from the same attack/defense formula during registration, and profiles whose explicit formula damage would exceed `max_hp` fail closed instead of being silently capped
- `level` / `rank` are now profile-owned metadata for later mob presentation, reward, or formula slices: built-in `training_dummy` and `practice_mob` default to `level = 1` and `rank = 0`, registered profiles preserve explicit values, omitted registered-profile `level` canonicalizes to the same bootstrap level `1`, and omitted registered-profile `rank` remains `0`
- runtime static-actor snapshots now expose the resolved profile presentation metadata as `combat_level` and `combat_rank` so loopback introspection, map/visibility snapshots, and later presentation slices can inspect the effective defaults without re-resolving the profile name; current HP mutation, reward payout, target carriers, and respawn timing still do not read those fields
- runtime code now has a narrow registration seam for additional bootstrap combat profiles with those same defaults, so later authored profiles can be introduced without hard-coding every new name into target/attack/respawn validation
- registered profile defaults are used by the same shared-world target/attack/death/respawn loop as built-in profiles: target selection starts from the registered `max_hp`, accepted normal attacks apply the registered attack/defense formula, HP percent is derived from that registered max, spawn-backed deaths can resolve the registered profile's reward descriptor, the dead timer uses the registered `respawn_delay`, and the rebuild restores the actor to the registered full HP
- registered profile names also use the same first aggro-lite ownership gate as built-in spawn-backed practice mobs: once the first owner lands an accepted hit in the current live loop, fresh third-party `TARGET` attempts fail closed until the existing engagement reset boundaries release or rebuild that actor
- registered profile names are immutable for the lifetime of the current process: registration fails closed when the name is blank, has non-canonical surrounding whitespace, is not a lowercase ASCII snake-case identifier (`[a-z][a-z0-9_]*`), names a built-in bootstrap profile, already exists, has neither a legacy `damage_per_normal_attack` value nor an explicit formula `attack_value`, supplies both legacy damage and explicit formula values that disagree after canonicalization, has invalid HP/formula/respawn defaults after canonicalization, has effective `damage_per_normal_attack > max_hp`, has explicit formula damage greater than `max_hp`, or carries an invalid reward descriptor
- `gamed` exposes a loopback-only operator profile endpoint for process-local profile authoring and inspection:
  - `GET /local/static-actor-combat-profiles`
  - returns the deterministic sorted list of built-in and registered combat-profile defaults, including derived `damage_per_normal_attack`, formula stats, presentation metadata, `respawn_delay_ms`, and cloned/sorted reward descriptors using stable snake-case `death_reward` JSON keys (`experience`, `gold`, `drop_vnums`)
  - `POST /local/static-actor-combat-profiles`
  - JSON fields: `profile`, `max_hp`, optional `damage_per_normal_attack`, optional formula fields `attack_value` / `defense_value`, optional presentation `level` / `rank`, `respawn_delay_ms`, and optional `death_reward` with `experience`, `gold`, and `drop_vnums`
  - success returns the canonicalized profile defaults, including derived `damage_per_normal_attack` and sorted/deduplicated reward drops
  - content-bundle canonicalization now snapshots custom registered profiles referenced by `spawn_groups` and `static_actors` in the top-level `combat_profiles` array, including formula stats, presentation metadata, respawn delay, and death-reward defaults, so exported authored combat content is self-describing for local QA instead of depending only on process-local profile registration state
  - built-in profile names are intentionally omitted from `combat_profiles` because their defaults are runtime-owned bootstrap constants, while custom profiles used by either authored collection are deduplicated and sorted by profile name
  - invalid JSON, unknown fields, non-loopback callers, built-in/duplicate/invalid profile names, profile names with surrounding whitespace, invalid formula defaults, invalid respawn delay, and invalid reward descriptors fail closed without registration
- that registration seam is still process-local operator tooling; content-bundle import/export now carries deterministic `combat_profiles` snapshots for custom authored profiles, but runtime still rejects malformed profile definitions and never canonicalizes padded profile names into a different key
- dots, spaces, hyphens, uppercase letters, and leading digits are intentionally rejected for combat profile names so profile identifiers stay distinct from authored `spawn_group_ref` values such as `practice.mob_alpha` and remain safe to compare as stable runtime selectors

### Runtime owns
- live entity IDs / VIDs
- current HP, dead/live state, and pending respawn bookkeeping
- the act of removing and recreating the visible runtime actor after death

This means respawn remains **server-driven runtime behavior**, but the runtime now knows *what to recreate and where* because the authored spawn group owns that identity and placement.

## Respawn rule for the first content-loaded combatant

The first spawn-group contract keeps respawn deliberately narrow:
- death still follows `non-player-death-respawn-bootstrap.md`
- respawn is still server-driven, not client-requested
- the recreated actor returns at the authored spawn-group position
- the recreated actor uses the authored `combat_profile`, or the default bootstrap `practice_mob` profile when the authored group omits that field
- the live runtime actor after respawn is a fresh instance of the same authored spawn group, not persistence resurrecting an old runtime entity ID

What is **not** yet frozen here:
- per-group custom respawn delays
- conditional spawn windows
- pack-wide synchronized respawn
- scripted on-death / on-respawn hooks

## Validation rules

The first content contract should fail closed when:
- `ref` is empty, duplicated, or not in the canonical dotted lowercase form `[a-z][a-z0-9_]*(.[a-z][a-z0-9_]*)+`
- `name` is empty after trimming whitespace
- `map_index` is `0`
- `race_num` is `0`
- `combat_profile` is unknown when provided; an omitted profile is canonicalized to the bootstrap `practice_mob` profile for this first one-spawn-profile contract
- coordinates are malformed for the current bundle schema
- reward scalar values overflow the current bootstrap point-change carrier, or `reward_drop_vnums` contains `0` or duplicate drop vnums

Import should reject malformed spawn groups before mutating live runtime state. The bundle canonicalization path now keeps spawn-group names explicit instead of synthesizing them from `ref`, rejects duplicate or non-canonical `ref` values without trimming the authored identifier into a different key, and preserves the prior authored/runtime snapshot when validation fails.

Runtime static-actor snapshots are also part of this contract because export, persistence rollback, map/visibility introspection, and respawn/rebuild code all round-trip through the same snapshot shape. A materialized spawn-group actor must therefore preserve its authored `spawn_group_ref` and normalized `combat_profile` in the live runtime snapshot, not just in the initial content-bundle record or file-backed store.

## Content bundle operator/runtime boundary

The bootstrap content-bundle surface uses the same top-level `spawn_groups` collection for export and import through the local operator bundle endpoint.

Current runtime rules:
- spawn-backed live actors export as `spawn_groups`, not as ordinary `static_actors`
- importing a bundle with `spawn_groups` materializes one runtime static actor per group with the authored `spawn_group_ref`
- the imported actor uses the authored placement, `race_num`, and normalized `combat_profile`
- if static-actor persistence fails after interaction definitions have already been replaced, import fails closed and restores the previously exported content bundle before reporting failure
- the operator endpoint remains loopback-only bootstrap tooling; it is not a gameplay packet or public API

This keeps authored attackable spawn content distinct from hand-authored visible/static actor content while still letting local QA export and re-import the current bootstrap content bundle deterministically.

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
- broader hostile retaliation or aggro-lite behavior beyond the first fresh-third-party `TARGET` gate, the first same-target `250ms` normal-attack cadence window, plus one sustained delayed self-only server-origin retaliation cadence at a time
- random spawn selection from a pool
- random loot tables, broader kill rewards, or corpse gameplay
- authored interaction metadata on attackable spawn groups
- migrations from old static-actor records into spawn groups

The first owned hostile post-hit reaction is intentionally tiny:
- once a visible content-loaded practice mob from `spawn_groups` accepts its first authoritative hit, fresh third-party `TARGET` attempts now fail closed until the existing death / respawn reset boundary unless the engaged owner's retaliation-driven `0`-HP death clears that engagement first
- that same first authoritative hit also clears any other session's already-selected shared-world combat-target ownership for that same mob and queues one self-only `GC TARGET(0, 0)` clear to each still-live affected third party, so a third party who preselected it before the owner hit cannot keep or visually retain a stale target-selection bypass before later `ATTACK` or fresh `TARGET` retries stay blocked until the owned release boundary is reached
- repeated normal `ATTACK` attempts now also obey one fixed server-owned `250ms` session-local cadence window; denied attempts inside the window stay silent and do not mutate HP or retaliation state, including attempts made immediately after retargeting to another visible practice mob
- while that practice mob stays alive, each accepted owner-side normal hit now also appends one immediate self-only `GC POINT_CHANGE` HP decrement to the engaged player's outgoing success frames
- content-loaded `practice_mob` actors now use the same owned normal-attack HP mutation path as `training_dummy`; each accepted normal hit decrements the live runtime HP by the profile's fixed bootstrap damage, clamps at `0`, and emits the same deterministic HP-percent result used by the target/point-change slices
- the first accepted live owner hit also arms one delayed self-only `GC POINT_CHANGE` follow-up beat after `1s`; it arrives through the pending server-frame path even if the owner sends no second `ATTACK`
- while that same engagement remains live, each delayed beat that fires automatically arms the next one after the same fixed delay, so the cadence is now independent from later client attack frames
- that owner-side retaliation point-loss now clamps at the current bootstrap HP floor too: neither the immediate hit-triggered tick nor the delayed follow-up cadence can drive the owner's visible HP below `0`, and once `0` is reached the current slice simply stops further retaliation point-loss without yet claiming broader player-death choreography
- those immediate and delayed owner-side retaliation point-loss beats are currently live-runtime only for that engaged selected session: they do **not** write the persisted account snapshot, and later position-only persistence helpers (`MOVE`, `SYNC_POSITION`, or transfer rebootstrap saves), successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves, plus non-point-bearing slash `/inventory_move` and merchant-buy saves now keep their coordinate, authored use/equip-effect point delta + consumed or carried/equipped item state, carried-slot, or purchase state without overwriting that pre-retaliation point value, so a fresh `/phase_select` re-entry or reconnect rebuilds from the pre-retaliation point value plus any later owned use/equip delta until broader player-death persistence or respawn semantics are owned
- when either the immediate retaliation tick or a delayed follow-up beat reaches that owner-side `0`-HP floor, the current slice now emits one self-only `GC DEAD(owner_vid)` before also clearing the stale engaged target with one self-only `GC TARGET(0, 0)`, and currently visible peers also receive one queued `GC DEAD(owner_vid)` while broader player-death semantics remain out of scope
- when that same owner-side `0`-HP floor is reached while the engaged practice mob still remains alive, the dead owner also stops holding that mob's aggro-lite ownership gate, so another visible live session may freshly `TARGET` the same still-live mob without waiting for owner disconnect or mob death / respawn
- once that retaliation floor has already reached `0`, later combat `TARGET` and normal `ATTACK` attempts from that same engaged owner against visible practice mobs also fail closed instead of continuing to reacquire or mutate runtime dummy HP before broader player-death semantics exist
- once that retaliation floor has already reached `0`, later peer-originated exact-name `WHISPER` requests aimed at that same still-connected owner also fail closed before queued target delivery or a synthetic `WHISPER_TYPE_NOT_EXIST` fallback can run
- once that retaliation floor has already reached `0`, later peer-originated `CHAT` requests with types `TALKING`, `PARTY`, `GUILD`, and `SHOUT` still return the live sender's ordinary self echo, but queued peer delivery skips that zero-HP owner recipient entirely under the current bootstrap chat-routing rules
- once that retaliation floor has already reached `0`, later owner-side `MOVE` and `SYNC_POSITION` attempts also fail closed before live position mutation, visibility rebuilds, queued fanout, or coordinate persistence can run
- once that retaliation floor has already reached `0`, later owner-side carried item-drop and carried gold-drop attempts also fail closed before inventory or gold mutation, ground-drop registration, queued visibility frames, or persistence can run
- once that retaliation floor has already reached `0`, later owner-side slash `/inventory_move` attempts also fail closed before carried-slot mutation can run
- once that retaliation floor has already reached `0`, later owner-side slash `/equip_item` and `/unequip_item` attempts also fail closed before carried/equipped item movement, self appearance refresh, or template-backed point mutation can run
- the runtime currently keeps at most one pending delayed follow-up beat at a time for that engaged owner/target pair, so accepted hits while one is already pending do not stack, accelerate, or reset the current delayed-retaliation timer yet
- if the owning live session disappears, clears target intent by movement / sync range loss, replaces target intent with a fresh `TARGET` on another visible practice mob, or the engaged actor dies / rebuilds before that delay expires, the queued follow-up beat fails closed, current cadence stops, and the abandoned still-live mob's aggro-lite gate is released immediately instead of leaving it orphan-locked forever
- if operator/runtime mutation or bundle replacement removes a currently selected live practice mob outright, visible sessions still receive the ordinary static-actor `CHARACTER_DEL`, and any session that still had that mob selected also receives one queued self-only `GC TARGET(0, 0)` so stale target ownership does not survive the runtime-removal boundary
- a same-socket `/quit`, `/logout`, or `/phase_select` now counts as that live-session disappearance boundary immediately in the current bootstrap slice, and abrupt session close does too: each path removes the owner from shared-world visibility, cancels any pending delayed follow-up beat, and releases the current aggro-lite target gate before any later disconnect or fresh bootstrap finishes; `/quit` still stays in `GAME` long enough to return its self `CHAT_TYPE_COMMAND quit` delivery, `/logout` continues to transition toward close, `/phase_select` returns to character select while any later bootstrap still requires a fresh `TARGET`, and close tears the session down without a compensating gameplay packet
- that first gate still does **not** imply movement, pathing, pack AI, or a broader aggro system beyond this fixed-delay owner-only cadence

## Success definition

After this document lands, the repository should be able to say:
- there is now one project-owned authored content seam for attackable non-player spawns: `spawn_groups`
- the first spawn group is intentionally size `1`, stationary, and combat-profile driven
- authored content now has a stable way to say which combatant should exist, where it should appear, which `combat_profile` it should use, and which deterministic EXP/gold/drop descriptor should apply on its killing hit
- respawn ownership is no longer implied to come from ad hoc runtime registration; it is conceptually anchored to the authored spawn-group `ref`
- one content-authored practice mob can now be imported through `spawn_groups`, fight using the owned `training_dummy` combat profile, rebuild after death through the existing server-driven respawn loop, reject fresh third-party `TARGET` attempts after its first accepted hit while that engaged owner still lives, proactively clear any stale preselected third-party target with one self-only `GC TARGET(0, 0)` when that same first hit establishes or preserves engagement, release that same gate again if retaliation kills the owner before the mob dies, apply one fixed same-target `250ms` normal-attack cadence gate, one immediate self-only owner HP decrement per accepted live hit, one sustained delayed self-only server-origin follow-up cadence at a time, runtime-only retaliation point-loss that does not yet persist across fresh `/phase_select` re-entry or reconnect and now also stays out of later position-only `MOVE` / `SYNC_POSITION` / transfer rebootstrap saves, successful slash `/use_item`, carried-slot `ITEM_USE`, `/equip_item`, and `/unequip_item` saves, plus non-point-bearing slash `/inventory_move` and merchant-buy saves while those helpers still persist coordinates, authored use/equip-effect point delta + consumed or carried/equipped item state, carried-slot state, or purchased item/gold state, self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` and one visible-peer `GC DEAD(owner_vid)` fanout when that retaliation floor reaches `0` HP, treat same-socket `/quit`, `/logout`, and `/phase_select` plus abrupt session close as immediate owner-disappearance boundaries for queued delayed retaliation + aggro release, also release the abandoned still-live mob immediately when movement / sync clears target intent or a fresh `TARGET` retargets another visible practice mob, close an already-open merchant window there with one self-only `GC::SHOP END`, and fail-closed owner-side combat `TARGET` / `ATTACK`, owner `MOVE` / `SYNC_POSITION`, owner static-actor `INTERACT`, owner merchant-buy attempts, owner carried gold-drop attempts, owner slash `/use_item` and carried-slot `ITEM_USE`, `/inventory_move`, `/equip_item`, and `/unequip_item` attempts, owner pee...
