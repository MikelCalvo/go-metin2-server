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
- the full gameplay meaning of every non-zero `attack_type` value beyond the first narrow bootstrap ownership boundary
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

The first owned combat request is now frozen exactly as:
- name: `ATTACK`
- direction: client -> server
- phase: `GAME`
- header: `0x0401`
- payload length: `7`
- status: documented and codec-owned in `internal/proto/combat`

The exact payload layout is:
1. `uint8 attack_type`
2. `uint32 target_vid` (little-endian)
3. `uint8 crc_proc_piece`
4. `uint8 crc_file_piece`

What is frozen now about those fields:
- the first live dummy attack path accepts only `attack_type = 0` (`normal attack`) in this slice
- `target_vid` is the wire-visible target identity the client places in the request
- `crc_proc_piece` and `crc_file_piece` are currently owned as exact trailing raw bytes in the codec, but their higher-level validation role remains intentionally narrow in the clean-room runtime for now

This exact codec ownership matters because the next flow slices no longer need to guess the attack header or open-code a one-off byte layout inside `internal/minimal`.

## Active-target prerequisite

The first owned attack-intent path is intentionally target-relative rather than free-form.

Even though the exact wire request carries a `target_vid`, the bootstrap runtime contract still treats that field as **subordinate to the currently selected combat target** rather than as permission to attack an arbitrary visible actor.

An `ATTACK` request is only eligible when all of the following are true:
- the session is already in `GAME`
- the session still owns a selected live character
- that live character currently holds one active combat target from the existing `TARGET` selection contract
- the request uses `attack_type = 0` for the first normal-attack bootstrap path
- the request `target_vid` exactly matches the session's currently selected combat target
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
1. `TARGET(target_vid > 0, hp_percent = 100)` — selected live dummy with fresh full bootstrap HP on first owned selection
2. `TARGET(target_vid > 0, hp_percent = updated)` — same selected dummy after accepted bootstrap attack-driven HP changes
3. `TARGET(0, 0)` — selected target cleared or no longer valid

## Relationship to later HP / death work

This document now freezes the first deterministic bootstrap HP mutation, but it still does **not** yet claim that death or respawn are implemented.

The current owned bootstrap combat state is intentionally tiny:
- visible `training_dummy` combat state is runtime-owned and starts at `10` HP
- each accepted bootstrap normal attack decrements the dummy by `1` HP
- the visible refresh stays on server `TARGET(target_vid, hp_percent)` using the current runtime HP converted to percent in `10`-point steps (`100`, `90`, `80`, ...)
- until the first death slice exists, this bootstrap loop clamps at the minimum live floor `1 / 10%` instead of claiming corpse/death choreography too early

What this still freezes about the **visible state carrier** for later slices:
- accepted later attack-driven HP refreshes should continue preferring server `TARGET` with the same selected `target_vid` plus the updated `hp_percent`
- target loss, invalidation, death cleanup, reconnect cleanup, transfer cleanup, or reclaim cleanup should prefer the zero-target `TARGET(0, 0)` companion before introducing a new clear-target family
- when subject movement or sync updates make the selected dummy leave current visibility or the bootstrap combat band, the runtime should proactively clear the active target with one self-only `TARGET(0, 0)` companion

If future captures or tests prove this carrier insufficient, the repository may add a richer combat packet family later.
But the next slices should begin from this smaller contract first.

## Repeated-hit loop and runtime-only HP ownership

The current bootstrap repeated-hit rule is now frozen as narrowly as possible:
- a visible `training_dummy` starts the live combat loop at authored/bootstrap max HP
- each accepted normal attack against the still-selected dummy decrements current live HP exactly once by the current bootstrap step
- the server reuses self-only `GC TARGET(target_vid, hp_percent)` after each accepted hit so the same selected target surface shows the updated percentage
- re-selecting that same still-visible dummy during the same live world runtime should return the current runtime-owned `hp_percent`, not silently recreate full HP on every request

The ownership rule is equally important:
- dummy HP belongs to shared-world runtime state, not to account or character persistence
- accepted dummy hits must not write inventory, equipment, player points, or any other character save payload as a side effect of combat alone
- this document does **not** yet freeze whether a reconnect, transfer, or future world rebuild should preserve or recreate dummy HP; it only freezes that the current bootstrap loop is runtime-owned and non-persistent

## Failure semantics

The first owned attack-intent path must stay fail-closed.

An `ATTACK` request must fail closed when any of these are true:
- wrong phase
- malformed codec payload
- no selected live character exists
- no active combat target is currently bound to the session
- the request uses a non-normal bootstrap `attack_type`
- the request `target_vid` does not match the session's active combat target
- the selected target is no longer visible
- the selected target is no longer a `training_dummy`
- the selected target is no longer within the current bootstrap combat band
- the session already lost authoritative live ownership because another session reclaimed the same character

The current visible failure expectations are intentionally narrow:
- malformed or wrong-phase requests may still stop at codec/flow rejection without a visible combat packet
- plain denied attack attempts do not yet require chat spam, peer fanout, or richer combat-result frames
- when runtime state already held a previously selected combat target and subject movement/sync makes that target invisible or out of the current combat band, the preferred first visible reset companion is one self-only `TARGET(0, 0)` plus local active-target cleanup

## Ownership and lifecycle rule

The first owned attack-intent contract must inherit the existing shared-world ownership model:
- attack authority belongs to the current live selected-character session
- stale reclaimed sockets must not authoritatively damage runtime-owned dummy HP, clear or replace the live owner's selected combat target, or queue combat-visible refresh frames to the replacement owner
- transfer rebootstrap, same-socket fresh bootstrap re-entry, and reconnect now clear session-local active combat target ownership before later attacks can proceed again
- non-player HP/dead state belongs to runtime world ownership, not to character persistence

Only some lifecycle edges are owned so far.
This document now freezes movement/sync invalidation plus fresh bootstrap/rebootstrap cleanup, while later combat slices still need to align the remaining lifecycle edges with the same runtime ownership model instead of creating a second combat-only ownership model.

## Explicit unknowns still left for the next RED

The next flow/gameplay slices still need to prove or freeze:
- whether the runtime should validate or currently only preserve the two trailing raw CRC bytes
- the exact first timing/rate rule for repeated attack attempts
- exactly which remaining lifecycle edges (death, actor replacement) should proactively emit `TARGET(0, 0)` instead of only failing closed

Those unknowns are deliberate.
The codec now owns the exact wire shape, but the gameplay contract is still intentionally narrower than full combat semantics.

## Explicit non-goals

This slice does **not** yet freeze:
- the final gameplay meaning of every `attack_type` value
- final damage formulas beyond the current bootstrap `1` HP decrement
- miss/crit/block results
- death or respawn packet choreography
- hostile retaliation
- player-vs-player attack semantics
- skills, buffs, debuffs, or status effects

## Success definition

After this document lands, the repository should be able to say:
- the next combat ingress is no longer vague; `ATTACK` is frozen exactly as client -> server header `0x0401`
- the project now owns the first clean-room `ATTACK` codec layout: `attack_type`, `target_vid`, `crc_proc_piece`, `crc_file_piece`
- the first live dummy attack path accepts only `attack_type = 0` and keeps gameplay target-relative by requiring the request `target_vid` to match the active selected combat target
- the first accepted bootstrap attack now mutates runtime-owned `training_dummy` HP deterministically from `10` downward in `1`-HP steps while reusing self-only `GC TARGET(target_vid, hp_percent)` as its visible success refresh
- accepted reselection of the same damaged dummy reuses the same current runtime `hp_percent` instead of resetting the visible target state back to `100`
- subject movement/sync that makes the selected dummy leave current visibility or the bootstrap combat band now proactively emits one self-only `GC TARGET(0, 0)` and clears the session-local active target
- transfer rebootstrap, same-socket fresh bootstrap re-entry, and reconnect now clear the session-local active target too, but the first owned contract keeps those lifecycle resets silent instead of claiming a visible clear-target packet
- duplicate-live reclaim now inherits the same shared-world hardening model as movement, whisper, item use, and merchant seams: stale `TARGET` / `ATTACK` packets fail closed and cannot mutate runtime dummy HP or the replacement owner's target state
- the first owned clear-target representation is now `GC TARGET(0, 0)`
- later HP refreshes should try to stay on the same `GC TARGET(target_vid, hp_percent)` carrier before claiming richer combat packets
