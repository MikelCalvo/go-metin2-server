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
- loot, EXP, quest rewards, drops, or corpse interactions
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
  "ref": "practice.training_dummy_a",
  "name": "Training Dummy A",
  "map_index": 42,
  "x": 1775,
  "y": 2875,
  "race_num": 20350,
  "combat_profile": "training_dummy"
}
```

In bundle form, the future authored surface is therefore:

```json
{
  "spawn_groups": [
    {
      "ref": "practice.training_dummy_a",
      "name": "Training Dummy A",
      "map_index": 42,
      "x": 1775,
      "y": 2875,
      "race_num": 20350,
      "combat_profile": "training_dummy"
    }
  ]
}
```

## Field meanings

The first bootstrap spawn-group contract freezes these fields:
- `ref`
  - stable authored identifier for the spawn group
  - unique within the bundle
  - this is the authored identity that future runtime respawn ownership binds to
- `name`
  - optional/operator-friendly display label
  - may surface in debugging, QA, or future operator tooling
- `map_index`
  - the effective bootstrap map where the combatant should spawn
- `x`, `y`
  - authored world coordinates for the spawn point
- `race_num`
  - the bootstrap non-player class/template identifier already used by static actors
- `combat_profile`
  - required authored combat metadata selector
  - `training_dummy` is the first owned value

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
- combat-profile selection (`combat_profile`)

### Combat profile owns
- combat defaults and rules shared by authored actors using that profile
- for the current bootstrap profile, that includes the existing training-dummy HP/death/respawn semantics already frozen elsewhere

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
- the recreated actor uses the authored `combat_profile`
- the live runtime actor after respawn is a fresh instance of the same authored spawn group, not persistence resurrecting an old runtime entity ID

What is **not** yet frozen here:
- per-group custom respawn delays
- conditional spawn windows
- pack-wide synchronized respawn
- scripted on-death / on-respawn hooks

## Validation rules

The first content contract should fail closed when:
- `ref` is empty or duplicated
- `map_index` is `0`
- `race_num` is `0`
- `combat_profile` is missing or unknown
- coordinates are malformed for the current bundle schema

Import should reject malformed spawn groups before mutating live runtime state.

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
- loot tables, kill rewards, or corpse gameplay
- authored interaction metadata on attackable spawn groups
- migrations from old static-actor records into spawn groups

The first owned hostile post-hit reaction is intentionally tiny:
- once a visible content-loaded practice mob from `spawn_groups` accepts its first authoritative hit, fresh third-party `TARGET` attempts now fail closed until the existing death / respawn reset boundary
- repeated normal `ATTACK` attempts against that same live selected target snapshot now also obey one fixed server-owned `250ms` cadence window; denied attempts inside the window stay silent and do not mutate HP or retaliation state
- while that practice mob stays alive, each accepted owner-side normal hit now also appends one immediate self-only `GC POINT_CHANGE` HP decrement to the engaged player's outgoing success frames
- the first accepted live owner hit also arms one delayed self-only `GC POINT_CHANGE` follow-up beat after `1s`; it arrives through the pending server-frame path even if the owner sends no second `ATTACK`
- while that same engagement remains live, each delayed beat that fires automatically arms the next one after the same fixed delay, so the cadence is now independent from later client attack frames
- that owner-side retaliation point-loss now clamps at the current bootstrap HP floor too: neither the immediate hit-triggered tick nor the delayed follow-up cadence can drive the owner's visible HP below `0`, and once `0` is reached the current slice simply stops further retaliation point-loss without yet claiming broader player-death choreography
- when either the immediate retaliation tick or a delayed follow-up beat reaches that owner-side `0`-HP floor, the current slice now emits one self-only `GC DEAD(owner_vid)` before also clearing the stale engaged target with one self-only `GC TARGET(0, 0)` while broader player-death semantics remain out of scope
- once that retaliation floor has already reached `0`, later combat `TARGET` and normal `ATTACK` attempts from that same engaged owner against visible practice mobs also fail closed instead of continuing to reacquire or mutate runtime dummy HP before broader player-death semantics exist
- the runtime currently keeps at most one pending delayed follow-up beat at a time for that engaged owner/target pair, so accepted hits while one is already pending do not stack, accelerate, or reset the current cadence timer yet
- if the owning live session disappears, clears or replaces target intent, or the engaged actor dies / rebuilds before that delay expires, the queued follow-up beat fails closed and the current cadence stops instead of leaving the mob orphan-locked forever
- that first gate still does **not** imply movement, pathing, pack AI, or a broader aggro system beyond this fixed-delay owner-only cadence

## Success definition

After this document lands, the repository should be able to say:
- there is now one project-owned authored content seam for attackable non-player spawns: `spawn_groups`
- the first spawn group is intentionally size `1`, stationary, and combat-profile driven
- authored content now has a stable way to say which combatant should exist, where it should appear, and which `combat_profile` it should use
- respawn ownership is no longer implied to come from ad hoc runtime registration; it is conceptually anchored to the authored spawn-group `ref`
- one content-authored practice mob can now be imported through `spawn_groups`, fight using the owned `training_dummy` combat profile, rebuild after death through the existing server-driven respawn loop, and reject fresh third-party `TARGET` attempts after its first accepted hit while also applying one fixed same-target `250ms` normal-attack cadence gate, one immediate self-only owner HP decrement per accepted live hit, one sustained delayed self-only server-origin follow-up cadence at a time, self-only `GC DEAD(owner_vid)` plus self-only `GC TARGET(0, 0)` when that retaliation floor reaches `0` HP, and fail-closed owner-side combat `TARGET` / `ATTACK` rejection there, without claiming movement AI, loot, or packs
