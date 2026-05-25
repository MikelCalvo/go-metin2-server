# Non-Player Reward Bootstrap

This document freezes the first deliberately tiny reward seam for bootstrap non-player combatants in `go-metin2-server`.

It sits on top of:
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `content-spawn-groups-bootstrap.md`

Those documents already freeze the target-relative `training_dummy` combat loop, zero-HP death, and timed respawn rebuild. This note answers the narrower follow-up question:

**What reward contract exists immediately after a bootstrap non-player death, before EXP, gold, drops, ownership rolls, or loot pickup are implemented?**

## Current scope

The first reward seam applies only to bootstrap static actors whose combat kind/profile is currently `training_dummy`.

The owned result is intentionally rewardless:
- EXP: `0`
- gold: `0`
- drops: empty list

The selected static-actor attack seam now carries that descriptor on the accepted zero-HP edge. In other words, the final accepted `ATTACK` that marks a `training_dummy` as dead returns the same explicit rewardless descriptor to runtime callers while the visible wire choreography remains unchanged: `GC DEAD(vid)` plus selected-target clear, not EXP/gold/drop packets.

Unknown combat kinds fail closed and produce no reward result.

## Why freeze a zero reward first

The combat loop can now kill and respawn a practice mob, so later work needs a stable place to hang deterministic reward behavior. Freezing a rewardless seam first prevents death handling from silently growing ad-hoc EXP, gold, or drop side effects before those contracts are separately documented and tested.

This also keeps the existing training dummy truthful: it is a practice target used to prove target/attack/death/respawn behavior, not a gameplay reward source yet.

## Explicit non-goals

This slice does **not** yet freeze:
- EXP point types or level progression
- gold mutation or gold packet fanout
- item-drop packet creation, ownership, timeout, or pickup rules
- party reward distribution
- quest credit or kill counters
- corpse interaction

## Implementation contract

Runtime helpers that need combat defaults or death rewards should ask the world-runtime combat-profile seam instead of inlining constants in session code.

For the current bootstrap runtime:
- `training_dummy` returns one supported profile-default record
- that record carries `max_hp = 10`, `damage_per_normal_attack = 1`, and `respawn_delay = 2s`
- the same record carries the current rewardless death descriptor: EXP `0`, gold `0`, and no drop vnums
- accepted non-lethal attacks keep their attack-result reward descriptor empty
- the accepted killing attack result exposes the profile's death reward descriptor to runtime code even when that descriptor is currently rewardless
- the descriptor has an explicit `Empty()` predicate so later EXP/gold/drop work can distinguish a deliberately empty reward from a non-empty reward without duplicating channel checks at each call site
- unsupported combat kinds return `ok = false`
- reward/default data remains runtime/configuration owned; it is not character persistence

## Success definition

After this slice, the repository can truthfully say:
- non-player death now has a dedicated reward seam
- the first `training_dummy` reward contract is explicitly rewardless
- later EXP/gold/drop slices have a small helper, attack-result handoff field, and protocol note to extend without changing the death/respawn choreography
