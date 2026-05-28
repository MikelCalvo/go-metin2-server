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

Narrow EXP, gold, and item-drop descriptors are now also owned for bootstrap practice-mob experiments:
- accepted EXP/gold killing hits apply the descriptor to the selected player only
- the account snapshot is persisted before an EXP/gold reward point-change frame is emitted
- the live player runtime is refreshed to the persisted point/currency value
- one self-only `PLAYER_POINT_CHANGE` for the EXP point (`POINT_EXP = 3`) and/or gold point (`POINT_GOLD = 11`) is appended after `GC DEAD(vid)` and `GC TARGET(0, 0)`
- drop descriptors append one self-only `GROUND_ADD` plus `OWNERSHIP` pair per configured drop vnum after any owned EXP/gold point-change frames
- mixed scalar+drop descriptors are now owned in one deterministic order: death, clear target, EXP point-change when present, gold point-change when present, then ground-add/ownership pairs in descriptor order
- authored `spawn_groups` may carry the same descriptor using `reward_experience`, `reward_gold`, and `reward_drop_vnums`; import/export and the static-actor snapshot store preserve those fields, and a content-loaded practice mob uses them on its killing hit without requiring a runtime-only test override
- the first drop rewards are runtime/world-owned ground presence at the killer's current position; they do not mutate character inventory or account persistence by themselves
- reward drops use the same pending ground-handle snapshot path as player drops, so `/local/maps` and relocation preview/transfer occupancy snapshots show them while they remain pending
- the killer can pick up a reward drop through the existing bootstrap ground-pickup path; pickup removes the ground item from both gameplay visibility and map-occupancy snapshots, emits `GROUND_DEL`, inventory `SET`/`UPDATE` frames as needed, emits `ITEM_GET`, persists the item in the selected character inventory, and rejects replayed pickup attempts fail-closed

Unknown combat kinds fail closed and produce no reward result.

## Why freeze a zero reward first

The combat loop can now kill and respawn a practice mob, so later work needs a stable place to hang deterministic reward behavior. Freezing a rewardless seam first prevents death handling from silently growing ad-hoc EXP, gold, or drop side effects before those contracts are separately documented and tested.

This also keeps the existing training dummy truthful: it is a practice target used to prove target/attack/death/respawn behavior, not a gameplay reward source yet.

## Explicit non-goals

This slice does **not** yet freeze:
- level progression from earned EXP
- default non-zero reward descriptors for authored `training_dummy` / `practice_mob` content
- level progression from mixed EXP/gold rewards
- item-drop timeout, loot ownership handoff beyond the bootstrap owner pickup path
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
- the player runtime has a narrow EXP/gold death-reward application helper; it mutates live session EXP/gold only, rejects drop-bearing descriptors, rejects unsigned/signed carrier overflow (including negative current EXP plus an oversized reward), and does not persist the account snapshot or emit any reward packet by itself
- the integrated game runtime owns the current persistence + packet edge for EXP/gold descriptors: save updated account points/currency, refresh the selected-player persisted snapshot, and append self-only `PLAYER_POINT_CHANGE` frames after the death/target-clear frames
- if EXP/gold reward persistence fails after an accepted killing edge, the runtime rolls live points/currency back to the previous selected-character snapshot, refreshes shared-world registration, preserves the already-owned death/target-clear frames, and omits reward `PLAYER_POINT_CHANGE` frames plus any later drop frames from that descriptor
- the integrated game runtime also owns item-drop descriptor edges: each configured drop vnum emits a deterministic ground item at the killer's current location plus an `OWNERSHIP` frame for the killer, registers that ground item in shared-world runtime state, and leaves inventory/account persistence unchanged until an explicit pickup request
- reward-drop registrations carry a non-zero runtime item ID derived from the ground VID so the existing ground-pickup inventory helper can accept and persist the pickup without special reward-only handling
- authored spawn groups may persist a descriptor with `reward_experience`, `reward_gold`, and `reward_drop_vnums`; omitted fields mean the rewardless descriptor, explicit non-zero fields round-trip through content bundle import/export and static-actor snapshots, and unsupported/invalid descriptors still fail closed during bundle validation before live runtime mutation
- mixed EXP/gold/drop descriptors are applied as one reward bundle after the accepted death edge; invalid drop entries or scalar persistence failure preserve the accepted death edge while omitting reward mutation/frames
- unsupported combat kinds return `ok = false`
- reward/default data remains runtime/configuration owned; it is not character persistence by itself

## Success definition

After this slice, the repository can truthfully say:
- non-player death has a dedicated reward seam
- the default `training_dummy` / `practice_mob` reward contract is explicitly rewardless
- EXP and gold descriptors can now be applied on the killing hit, persisted to the selected character, and reported with self-only `PLAYER_POINT_CHANGE` frames
- drop descriptors can now create one self-visible ground item reward per configured drop vnum without immediate inventory/account mutation, and the killer can later pick up that ground reward through the existing persisted ground-pickup path
- mixed scalar+drop descriptors now preserve deterministic reward ordering while keeping invalid-drop or save-failure fallbacks on the already-accepted death edge
- later reward slices have a small descriptor helper, attack-result handoff field, scalar application seam, and protocol note to extend without changing the death/respawn choreography
