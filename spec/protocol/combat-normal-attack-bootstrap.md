# Combat Normal Attack Bootstrap

This document freezes the first owned attack-intent and clear-target contract for `go-metin2-server`.

It sits on top of:
- `combat-training-dummy-bootstrap.md`
- `non-player-entity-bootstrap.md`
- `shared-world-peer-visibility.md`
- `runtime-reconnect-cleanup.md`

Those documents already freeze:
- one visible `training_dummy` target class addressed by client-visible `VID`
- the first self-only `GC TARGET` acknowledgement for accepted target selection
- the current visibility/range/runtime ownership rules that decide whether a dummy can stay targetable at all
- the existing reconnect/reclaim cleanup style that later combat slices must reuse instead of inventing separate ownership semantics

What this document adds is the next narrower question:

**What is the smallest honest attack-intent step the project can own next without pretending that full damage, death, aggro, or mob AI already exist?**

## Scope

This contract currently applies only to:
- one connected `GAME` session with a selected live character
- one active selected combat target already accepted through the existing `TARGET` selection path
- one currently visible in-range non-player actor still marked as `training_dummy`
- one immediate attack-intent request against that already selected target
- one tiny target-refresh surface that can still describe `current target`, `updated hp percent`, or `no active target`

This contract does **not** yet claim:
- the final exact legacy wire header or payload layout for the attack request
- combo chains, animation timing, attack speed, or projectile choreography
- richer attack-result packets, hit effects, floating damage numbers, or skill systems
- combat against player targets
- aggro, retaliation, patrol, or movement AI
- loot, EXP, death rewards, corpse state, or quest hooks
- final persistence rules for non-player combat state

## Why freeze attack intent before full combat

The repository already has a real target-selection slice:
- `TARGET` can now select one visible in-range `training_dummy`
- that selection already reuses shared-world visibility, map, and ownership rules
- the runtime already knows how to reject invisible, out-of-range, stale, or non-targetable candidates fail-closed

What is still missing is the next concrete step after target selection.

Without a written attack-intent contract, later slices would risk:
- inventing ad-hoc attack ingress straight inside `internal/minimal`
- coupling HP updates to a guessed packet layout too early
- introducing a separate clear-target packet family before proving the smaller reuse path is insufficient

So this document freezes the smallest next ownership boundary first.

## First owned attack-intent family

The next planned combat request is now frozen conceptually as:
- name: `ATTACK`
- direction: client -> server
- phase: `GAME`
- header: `TBD`
- status: planned, but now part of the owned project contract

Why `TBD` is still acceptable here:
- the project already distinguishes `planned` rows from fully wire-frozen rows in `packet-matrix.md`
- the next slice should write RED codec tests before claiming the final exact legacy byte layout
- the repository can still freeze the family name, scope, gating rules, and relationship to target ownership without inventing a fake exact header too early

What is frozen now is the behavioral role of this family:
- `ATTACK` is the first client-originated step after accepted target selection
- it is only valid while the session currently owns an active selected combat target
- it addresses the **current selected target**, not an arbitrary visible `VID` chosen independently of the target-selection contract
- later exact codec work may still discover trailing bytes or subfields, but those bytes must serve this same narrow attack-intent role

## Active-target prerequisite

The first owned attack-intent path is intentionally target-relative rather than free-form.

An `ATTACK` request is only eligible when all of the following are true:
- the session is already in `GAME`
- the session still owns a selected live character
- that live character currently holds one active combat target from the existing `TARGET` selection contract
- that selected target still resolves to a visible same-map `training_dummy`
- that selected target still passes the current bootstrap combat band

This keeps the first attack slice aligned with the already-owned `TARGET` path instead of creating a second competing target-identity model.

## First clear-target representation

The first owned clear-target companion is now frozen as a **reuse of the existing server -> client `TARGET` family**, not as a separate dedicated clear packet.

The working contract is:
- server -> client `TARGET` with `target_vid = 0`
- server -> client `TARGET` with `hp_percent = 0`
- combined meaning: **no active combat target remains bound to the session**

Why this reuse is the current preferred contract:
- the repository already owns `GC TARGET` as the smallest self-only target-state surface
- the same packet family can already describe `current target + hp percent`
- reusing it for `no target` avoids inventing a second clear-only family before tests prove a richer path is needed

So the first owned target-state surface is now intentionally tiny but expressive enough for three states:
1. `TARGET(target_vid > 0, hp_percent = 100)` — selected live dummy with full bootstrap HP percent
2. `TARGET(target_vid > 0, hp_percent = updated)` — same selected dummy after later accepted attack-driven HP changes
3. `TARGET(0, 0)` — selected target cleared or no longer valid

## Relationship to later HP / death work

This document does **not** yet claim that HP mutation, death, or respawn are already implemented.

What it freezes is the **visible state carrier** those later slices should try first:
- accepted later attack-driven HP refreshes should prefer reusing server `TARGET` with the same selected `target_vid` plus the updated `hp_percent`
- target loss, invalidation, death cleanup, reconnect cleanup, transfer cleanup, or reclaim cleanup should prefer the zero-target `TARGET(0, 0)` companion before introducing a new clear-target family

If future captures or tests prove this carrier insufficient, the repository may add a richer combat packet family later.
But the next slices should begin from this smaller contract first.

## Failure semantics

The first owned attack-intent path must stay fail-closed.

An `ATTACK` request must fail closed when any of these are true:
- wrong phase
- malformed codec payload
- no selected live character exists
- no active combat target is currently bound to the session
- the selected target is no longer visible
- the selected target is no longer a `training_dummy`
- the selected target is no longer within the current bootstrap combat band
- the session already lost authoritative live ownership because another session reclaimed the same character

The current visible failure expectations are intentionally narrow:
- malformed or wrong-phase requests may still stop at codec/flow rejection without a visible combat packet
- plain denied attack attempts do not yet require chat spam, peer fanout, or richer combat-result frames
- when runtime state already held a previously selected combat target but that target is now invalid, the preferred first visible reset companion is `TARGET(0, 0)`

## Ownership and lifecycle rule

The first owned attack-intent contract must inherit the existing shared-world ownership model:
- attack authority belongs to the current live selected-character session
- stale reclaimed sockets must not authoritatively damage or clear the live owner's target state
- reconnect, transfer, and teardown behavior should eventually clear target ownership using the same runtime cleanup style already frozen elsewhere
- non-player HP/dead state belongs to runtime world ownership, not to character persistence

This document does not implement those behaviors yet.
It freezes that later combat slices should align with the existing runtime lifecycle model instead of creating a second combat-only ownership model.

## Explicit unknowns still left for the next RED

The next codec/documentation slices still need to prove or freeze:
- the exact legacy client -> server `ATTACK` header
- the exact request payload bytes, if any, beyond the common frame envelope
- whether the first clean-room codec should expose any opaque trailing fields before their meaning is fully known
- whether accepted attack attempts need any additional tiny server packet besides `TARGET` refreshes to keep the client stable
- the exact first timing/rate rule for repeated attack attempts

Those unknowns are deliberate.
This document narrows the next work enough that the RED tests can be small without overstating wire certainty.

## Explicit non-goals

This slice does **not** yet freeze:
- exact attack codec bytes
- damage formulas
- miss/crit/block results
- death or respawn packet choreography
- hostile retaliation
- player-vs-player attack semantics
- skills, buffs, debuffs, or status effects

## Success definition

After this document lands, the repository should be able to say:
- the next combat ingress is no longer vague; it is a planned `ATTACK` family in `GAME`
- that family is now frozen as an attack against the **currently selected** target, not as a second arbitrary target-selection path
- the first owned clear-target representation is now `GC TARGET(0, 0)`
- later HP refreshes should try to stay on the same `GC TARGET(target_vid, hp_percent)` carrier before claiming richer combat packets
- the next RED can focus on exact codec ownership and accepted dummy attack gating without reopening target identity, clear-target semantics, or session-ownership rules
