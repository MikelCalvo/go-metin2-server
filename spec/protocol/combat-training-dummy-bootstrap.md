# Combat Training Dummy Bootstrap

This document freezes the first combat-preparation contract for `go-metin2-server`.

It sits on top of:
- `non-player-entity-bootstrap.md`
- `shared-world-peer-visibility.md`
- `inventory-equipment-bootstrap.md`
- `player-point-change-bootstrap.md`

Those documents already freeze:
- visible bootstrap non-player actors in the shared world
- the first owned inventory/equipment and point-refresh seams for the selected character
- the current topology/AOI visibility rules that decide whether a peer or static actor is targetable at all

What this document adds is the next narrower question:

**What is the smallest honest combat target-selection layer the project owns underneath the later attack, death, and respawn follow-up specs?**

The follow-up planned attack-intent and clear-target contract now lives in:
- `combat-normal-attack-bootstrap.md`

The follow-up planned death / respawn contract now lives in:
- `non-player-death-respawn-bootstrap.md`

This file intentionally stays scoped to target selection even though the end-to-end `training_dummy` loop now continues through those follow-up documents.

## Scope

This contract currently applies only to:
- one connected `GAME` session with a selected live character
- one currently visible non-player target represented by the same client-visible `VID` already used by bootstrap static-actor visibility
- one deliberately tiny target class: a visible `training_dummy`
- one bootstrap distance/range band used only for selecting that dummy as a future combat target
- transient live runtime target ownership for that selected session only
- later attack-intent slices that must reuse this same lookup and gating contract

It does **not** yet claim:
- real attack execution
- attack animations or timing
- damage formulas, hit results, or HP bars
- aggro, retaliation, AI, movement, or spawn groups
- loot, drops, or corpse gameplay; the first death / respawn boundary is now documented separately in `non-player-death-respawn-bootstrap.md`
- persistent target state across reconnects, transfers, or ownership reclaim
- target selection for other player characters
- final client HUD/reticle choreography beyond the minimal request boundary frozen here

## Why a training dummy first

The repository now already owns enough character/runtime state to prepare combat honestly:
- visible non-player actors already exist in the live bootstrap world
- target identity can already ride the same visible `VID` surface used by peer/static-actor visibility
- the selected character already owns minimal live inventory/equipment and the first self-only template-backed point changes
- topology/AOI helpers already decide whether an actor is visible under the active runtime policy

At the same time, several larger systems are still intentionally absent:
- real mob AI and spawn systems
- broader combat formulas and richer damage semantics
- loot, reward, and corpse gameplay systems beyond the separate bootstrap death/respawn contract
- richer target UI capture work

Because of those constraints, the next honest combat step is **target selection against a fixed visible training dummy**, not a full mob or damage loop.

## First owned targetable actor class

The first combat target is intentionally narrow:
- actor kind: visible bootstrap non-player actor
- gameplay class: `training_dummy`
- behavior today: stationary, passive, non-retaliating, and only interesting as a target candidate
- identity surface: the actor's current visible `VID`

This slice freezes the rule that **combat targetability is separate from interactability**:
- a visible NPC/actor being interactable does not automatically make it combat-targetable
- the first combat target path should only accept actors explicitly authored/runtime-marked with the `training_dummy` combat profile
- later slices may broaden targetable non-player classes without changing the core identity rule (`visible VID`)

The current authored metadata seam names that tag `combat_profile` and persists it through static-actor snapshots plus content-bundle import/export.
What is frozen here is the behavior contract and authored meaning of the `training_dummy` profile, not a promise that richer future profiles are already implemented.

## Planned request boundary

The first owned combat-preparation request is now frozen as:
- name: `TARGET`
- direction: client -> server
- header: `0x0A01`
- phase: `GAME`
- payload: little-endian `uint32 target_vid`

The minimal self-only acknowledgement companion is now also frozen as:
- name: `TARGET`
- direction: server -> client
- header: `0x0A10`
- phase: `GAME`
- payload: little-endian `uint32 target_vid` + `uint8 hp_percent`
- current bootstrap meaning: fresh accepted dummy selection starts at `hp_percent = 100`, while later accepted dummy attacks may reuse the same packet family with the current runtime-owned percentage

This contract now freezes the **family name, direction, phase, concrete wire headers, and the narrow request/ack payload shapes**.
The repo now owns:
- an exact `internal/proto/combat` codec for both directions
- `GAME`-phase flow dispatch for the request
- minimal runtime wiring that reuses the existing shared-world `AttemptStaticActorCombatTarget(...)` seam
- one accepted self-only `GC TARGET` ack for a visible in-range `training_dummy`

It does **not** yet freeze:
- a clear-target request shape
- a damage or hit-result packet family
- visible target-loss packets on transfer, reconnect, re-enter, reclaim, actor replacement, or death; visibility/range invalidation is now owned by the follow-up combat-normal bootstrap contract via self-only `GC TARGET(0, 0)`

## Target identity and visibility rule

The request target is the dummy actor's current client-visible `VID`.

That keeps the first combat path aligned with the already-owned shared-world/static-actor visibility surfaces and avoids inventing a second target-identity scheme before real attacks exist.

A target is eligible only when all of the following are true:
- the requesting session is already in `GAME`
- the requesting session still owns a selected live character
- that live character's current bootstrap HP is still above `0`; once the current practice-mob retaliation slice has already driven the owner to `0`, later combat `TARGET` attempts fail closed until broader player-death semantics are owned separately
- the target `VID` resolves to a currently visible non-player actor under the active topology/AOI policy
- that actor is currently marked as a `training_dummy`
- that actor is within the current bootstrap combat-selection range band

## Bootstrap range band

The first owned combat-selection range band is intentionally fixed and simple:
- distance gate: `300` world units
- measurement: planar world distance between the selected character and the dummy actor in the same effective map

Why freeze a fixed band now:
- it matches the current style of early bootstrap gating already used elsewhere for visible interactions
- it gives the next RED tests one deterministic allow/deny boundary
- it avoids inventing weapon-specific reach, pathfinding, or animation timing before the first target path even exists

Later combat slices may replace or widen that band, but only after a minimal end-to-end target-and-attack loop exists.

## Runtime ownership rule

Accepted target selection should remain transient live runtime state only.

This first contract intentionally expects:
- target ownership is per selected live session
- accepted target identity is not just the dummy `VID`; it also binds the current runtime snapshot behind that visible dummy until the session reselects it
- selecting a dummy does not mutate persistence, inventory, equipment, or points by itself
- dummy combat HP is world-runtime-owned state, not character/account persistence
- repeated accepted attacks may later mutate that live dummy HP without implying any player-save write
- selecting a dummy emits at most one self-only `GC TARGET` acknowledgement on accept
- selecting a dummy does not broadcast to peers
- selecting a dummy only prepares later attack-intent validation on that same live session
- a dummy at runtime-owned `0` HP is no longer eligible for accepted bootstrap target selection
- target ownership dies at fresh bootstrap/rebootstrap boundaries; transfer rebootstrap, same-socket `/phase_select` re-entry, and fresh reconnect all require a new accepted `TARGET` request before later attacks can proceed again

Visibility/range invalidation for an already selected dummy now lives in `combat-normal-attack-bootstrap.md` via the self-only `GC TARGET(0, 0)` companion.
The first death / respawn follow-up now lives in `non-player-death-respawn-bootstrap.md`.

## Runtime seam already owned before and after the packet path

The repository now owns one narrow runtime path end to end:
- `internal/worldruntime.StaticEntity` can carry optional combat-target metadata using the current `training_dummy` combat profile
- invalid authored combat profiles fail closed at the non-player directory boundary
- `internal/minimal/shared_world` owns the structured target-attempt validation seam for visible training dummies
- that seam checks subject ownership, visible-actor lookup by `VID`, fixed `300`-unit range gating, and targetable-class filtering
- `internal/proto/combat` now owns exact client/server `TARGET` codecs for the current request/ack pair
- `internal/game` now dispatches client `TARGET` in `GAME` and fail-closes malformed payloads or rejected runtime attempts
- the current runtime reply is still deliberately tiny: one self-only `GC TARGET` ack with `target_vid` and bootstrap-selected `hp_percent`, where a fresh dummy starts at `100` and later attack slices may reuse the same shape with a lower current runtime percentage
- no client-visible combat packet beyond that ack, no HUD state machine, and no attack execution is implied by this path alone

## Failure semantics

The current owned failure contract should stay minimal and fail closed:
- wrong phase -> existing flow rejection rules apply
- malformed payload -> codec/flow rejection applies
- subject has no live selected character -> request fails closed
- subject's current bootstrap HP is already `0` -> request fails closed
- target `VID` not found in current visible non-player state -> request fails closed
- target actor is visible but not marked `training_dummy` -> request fails closed
- target actor is visible and targetable but out of the `300`-unit range band -> request fails closed
- rejected attempts do not emit chat, peer fanout, persistence writes, or a compensating clear-target packet in this slice

This slice does **not** yet require:
- self-only info chat on deny
- visible target-clear packets
- peer-facing notifications
- special dummy state transitions

The next combat RED should therefore build on this request/ack seam without skipping the existing lookup, ownership, targetability, and range gating rules.

## Explicit non-goals

This slice does **not** yet freeze:
- broader attack implementation beyond target selection itself
- weapon swing or projectile choreography
- hit registration
- damage numbers or point/HP depletion
- target persistence across duplicate-live retry or reclaim
- selecting player characters as combat targets
- auto-acquire, tab-target cycling, or click-to-move behavior
- mobs, spawn groups, aggro, patrols, or scripted encounters

## Success definition

After this document and slice, the repository should be able to say:
- the first client-visible combat slice is no longer vague; it is a concrete `TARGET` request/ack path for one visible `training_dummy`
- the first combat target identity is the already-visible non-player `VID`
- the first owned target request family is `TARGET` in `GAME` with `0x0A01`
- the first owned self-only acknowledgement family is `TARGET` in `GAME` with `0x0A10`
- the first targetable actor class is a visible `training_dummy`, not every static actor or every NPC
- accepted in-range dummy selection now returns one self-only `GC TARGET` ack without dragging in attack execution, damage, aggro, or AI
- once owner-side practice-mob retaliation has already driven that live character to `0` HP, fresh combat `TARGET` attempts now fail closed too until broader player-death semantics are owned separately
- fresh bootstrap entry, transfer rebootstrap, and reconnect clear previously selected dummy target ownership so later attacks must reacquire intent with a new `TARGET` request
- rejected attempts still fail closed without chat spam, peer fanout, persistence writes, or clear-target choreography
- combat remains intentionally tiny, but the first honest target-selection request path is now frozen in both docs and tests
