# Non-Player Reward Bootstrap

This document freezes the first deliberately tiny reward seam for bootstrap non-player combatants in `go-metin2-server`.

It sits on top of:
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `content-spawn-groups-bootstrap.md`

Those documents already freeze the target-relative `training_dummy` combat loop, zero-HP death, and timed respawn rebuild. This note answers the narrower follow-up question:

**What reward contract exists immediately after a bootstrap non-player death, before EXP, drops, ownership rolls, or loot pickup are implemented?**

## Current scope

The first reward seam applies only to bootstrap static actors whose combat kind/profile is currently `training_dummy` / `practice_mob`.

The default authored result remains intentionally rewardless:
- EXP: `0`
- gold: `0`
- drops: empty list

The selected static-actor attack seam carries that descriptor on the accepted zero-HP edge. In other words, the final accepted `ATTACK` that marks a default `training_dummy` / `practice_mob` as dead returns an explicit rewardless descriptor to runtime callers while the visible death choreography remains `GC DEAD(vid)` plus selected-target clear.

Narrow EXP-only, gold-only, and first item-drop-only runtime descriptors are now also owned for bootstrap practice-mob experiments:
- accepted EXP/gold killing hits apply the descriptor to the selected player only
- the account snapshot is persisted before an EXP/gold reward point-change frame is emitted
- the live player runtime is refreshed to the persisted point/currency value
- one self-only `PLAYER_POINT_CHANGE` for the EXP point (`POINT_EXP = 3`) or gold point (`POINT_GOLD = 11`) is appended after `GC DEAD(vid)` and `GC TARGET(0, 0)`
- an item-drop-only descriptor appends one self-only `GROUND_ADD` plus `OWNERSHIP` pair per configured drop vnum after `GC DEAD(vid)` and `GC TARGET(0, 0)`
- the first drop rewards are runtime/world-owned ground presence at the killer's current position; they do not mutate character inventory or account persistence by themselves

Unknown combat kinds fail closed and produce no reward result.

## Why freeze a zero reward first

The combat loop can now kill and respawn a practice mob, so later work needs a stable place to hang deterministic reward behavior. Freezing a rewardless seam first prevents death handling from silently growing ad-hoc EXP, gold, or drop side effects before those contracts are separately documented and tested.

This also keeps the existing training dummy truthful: it is a practice target used to prove target/attack/death/respawn behavior, not a gameplay reward source yet.

## Explicit non-goals

This slice does **not** yet freeze:
- level progression from earned EXP
- default non-zero reward descriptors for authored `training_dummy` / `practice_mob` content
- mixed EXP/gold/drop descriptor application
- item-drop timeout, loot ownership handoff, or pickup reward rules beyond the immediate self-visible `GROUND_ADD` + `OWNERSHIP` pairs
- party reward distribution
- quest credit or kill counters
- corpse interaction

## Implementation contract

Runtime helpers that need combat defaults or death rewards should ask the world-runtime combat-profile seam instead of inlining constants in session code.

For the current bootstrap runtime:
- `training_dummy` and `practice_mob` return supported profile-default records
- those records carry `max_hp = 10`, `damage_per_normal_attack = 1`, and `respawn_delay = 2s`
- those default records carry the current rewardless death descriptor: EXP `0`, gold `0`, and no drop vnums
- accepted non-lethal attacks keep their attack-result reward descriptor empty
- the accepted killing attack result exposes the profile or runtime override death-reward descriptor to runtime code
- the descriptor has an explicit `Empty()` predicate so later EXP/gold/drop work can distinguish a deliberately empty reward from a non-empty reward without duplicating channel checks at each call site
- the descriptor has an explicit `Clone()` helper that deep-copies the drop-vnum list and normalizes empty drop lists to `nil`, so future non-zero drop-table slices do not accidentally share mutable reward slices across profile-default lookups or attack results
- the player runtime has a narrow EXP/gold death-reward application helper; it mutates live session EXP/gold only, rejects drop-bearing descriptors, rejects overflow, and does not persist the account snapshot or emit any reward packet by itself
- the integrated game runtime owns the current persistence + packet edge for EXP/gold descriptors: save updated account points/currency, refresh the selected-player persisted snapshot, and append one self-only `PLAYER_POINT_CHANGE` after the death/target-clear frames
- if EXP/gold reward persistence fails after an accepted killing edge, the runtime rolls live points/currency back to the previous selected-character snapshot, refreshes shared-world registration, preserves the already-owned death/target-clear frames, and omits the reward `PLAYER_POINT_CHANGE`
- the integrated game runtime also owns an item-drop-only descriptor edge: each configured drop vnum emits a deterministic ground item at the killer's current location plus an `OWNERSHIP` frame for the killer, registers that ground item in shared-world runtime state, and leaves inventory/account persistence unchanged
- mixed EXP/gold/drop descriptors currently preserve the accepted death edge while omitting reward mutation/frames until those richer channels are explicitly owned
- unsupported combat kinds return `ok = false`
- reward/default data remains runtime/configuration owned; it is not character persistence by itself

## Success definition

After this slice, the repository can truthfully say:
- non-player death has a dedicated reward seam
- the default `training_dummy` / `practice_mob` reward contract is explicitly rewardless
- EXP-only and gold-only descriptors can now be applied on the killing hit, persisted to the selected character, and reported with one self-only `PLAYER_POINT_CHANGE`
- drop-only descriptors can now create one self-visible ground item reward per configured drop vnum without mutating character inventory/account persistence
- unsupported mixed descriptors keep the already-accepted death edge visible while omitting reward mutation/frames until that reward channel is explicitly owned
- later EXP/drop/ownership slices have a small descriptor helper, attack-result handoff field, gold-only application seam, and protocol note to extend without changing the death/respawn choreography
