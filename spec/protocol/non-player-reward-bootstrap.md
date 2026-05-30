# Non-Player Reward Bootstrap

This document freezes the first deliberately narrow reward contract for bootstrap non-player combatants in `go-metin2-server`.

It sits on top of:
- `combat-normal-attack-bootstrap.md`
- `non-player-death-respawn-bootstrap.md`
- `content-spawn-groups-bootstrap.md`
- `player-point-change-bootstrap.md`

Those documents already freeze:
- the selected-target `ATTACK` loop for `training_dummy` / `practice_mob` combatants
- the zero-HP `DEAD(vid)` plus `TARGET(0, 0)` death edge
- authored `spawn_groups` with optional reward descriptor fields
- the existing self-only point-change and ground-item packet families reused here

## Scope

This contract currently applies only to an accepted killing hit against a runtime-owned bootstrap non-player combatant whose death edge is already committed.

The current reward descriptor is intentionally tiny:
- `reward_experience uint64`
- `reward_gold uint64`
- `reward_drop_vnums []uint32`

It does **not** yet claim:
- level-up choreography or stat recalculation
- party reward sharing or contribution splits
- randomized loot tables or probabilities
- corpse interaction, pickup ownership expiry, or public loot release
- quest credit, achievements, or scripted on-death hooks
- persisted non-player reward state across process restart

## Descriptor ownership

Reward descriptors can come from authored `spawn_groups` or from the live static-actor snapshot that was materialized from that authored content.
Standalone static actors that are not backed by a non-empty `spawn_group_ref` must remain rewardless; both the static snapshot store and the in-memory non-player directory reject reward metadata on those actors so rewards do not become a generic static-actor feature by accident.

The descriptor is not character persistence by itself.
It becomes player state only after the already-accepted death edge tries to apply the reward through the selected live session.

Default bootstrap `training_dummy` and `practice_mob` profile defaults remain rewardless unless content or tests explicitly author a non-zero descriptor.
Both profile-level default helpers are covered directly so a later combat-profile broadening cannot accidentally turn either bootstrap profile into an implicit reward source.

## Validation

Reward descriptors fail closed when:
- `reward_experience` or `reward_gold` exceeds the current signed 32-bit `PLAYER_POINT_CHANGE` carrier range (`2,147,483,647`)
- applying `reward_experience` or `reward_gold` would overflow the selected character's current signed 32-bit visible point / gold carrier
- any `reward_drop_vnums` entry is `0`
- any `reward_drop_vnums` entry is duplicated in the same descriptor
- a runtime-generated ground-item VID for a configured drop would be `0`
- multiple drops in the same descriptor would collide on the generated ground-item VID
- a configured drop would reuse an already-live ground-item VID

A live ground-item VID collision is intentionally treated as a drop-only failure, not as a combat rollback: the accepted killing hit still emits `DEAD(target_vid)` and the killer's `TARGET(0, 0)` clear, the pre-existing ground item remains registered, and the colliding reward drop emits no `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` frames.
If the same descriptor also carries valid EXP or gold, those scalar rewards still apply and emit their ordinary `PLAYER_POINT_CHANGE` frames; the colliding drop does not suppress independent scalar reward families.

The descriptor validator itself owns the static authoring checks before runtime kill handling begins:
- signed 32-bit scalar carrier maximums are accepted
- scalar values above that maximum are rejected
- zero-valued drop vnums are rejected
- duplicate drop vnums in one descriptor are rejected

Malformed reward descriptors must not roll back the already-accepted combat death edge.
They simply suppress reward mutation and reward frames for that kill.

## Killing-hit ordering

The killing hit keeps death choreography first.
Reward frames are appended only after:
1. `GC DEAD(target_vid)`
2. `GC TARGET(0, 0)` for the killer if that target was still selected

After those frames, successful reward feedback is ordered as:
1. optional EXP `GC PLAYER_POINT_CHANGE`
2. optional gold `GC PLAYER_POINT_CHANGE`
3. one `ITEM_GROUND_ADD` + `ITEM_OWNERSHIP` pair per configured drop, in descriptor order

This ordering keeps combat lifecycle visible before reward side effects.

## Scalar EXP and gold rewards

EXP and gold rewards reuse `GC PLAYER_POINT_CHANGE` as the visible self-only carrier.

Current rules:
- EXP uses the bootstrap experience point type already used by selected-character points
- gold uses the bootstrap gold point type
- each scalar reward is applied to the selected live player runtime first
- the updated selected-character account snapshot is saved before the corresponding point-change frame is appended
- if the player runtime rejects the scalar reward because the descriptor or resulting live values would overflow the signed 32-bit visible carriers, or if account persistence fails, the accepted death/clear frames remain, scalar reward frames are omitted, and the live EXP/gold scalar values stay at or roll back to their pre-reward values; other live runtime state such as the current in-world position must not be clobbered

## Drop rewards

Drop rewards reuse the existing bootstrap ground-item families:
- `ITEM_GROUND_ADD`
- `ITEM_OWNERSHIP`
- later pickup still uses the normal owned `ITEM_PICKUP` path, producing `ITEM_GROUND_DEL`, `ITEM_SET`, and `ITEM_GET` as appropriate

Ground reward registration is guarded by the same live-owner rule for both item-shaped and gold-shaped ground entries.
If the would-be owner is already at the current bootstrap `0` HP floor, registering either a ground item or ground gold fails closed and leaves no live ground occupancy behind.
Ground reward visibility fanout is also live-recipient guarded: dead visible peers do not receive `ITEM_GROUND_ADD` / `ITEM_OWNERSHIP` on registration and do not receive `ITEM_GROUND_DEL` when the ground entry is later removed.
This keeps death/restart cleanup from leaking new pickup surfaces or stale delete noise for recipients that are already dead.

Current rules:
- each configured drop spawns at the killer's current position
- each drop has count `1`
- each drop is owned by the killer's character name
- item drops are runtime ground items first; they do not mutate persisted inventory until an explicit pickup succeeds
- replayed pickup of the same ground VID fails closed after the first successful pickup removes it

## Respawn / lifecycle ownership

Reward descriptors belong to the authored spawn snapshot, not to the transient live HP/dead state.

Current rules:
- killing a rewarded practice mob may apply that descriptor once for the accepted death edge
- the timed respawn rebuild restores the same authored actor identity and preserves `spawn_group_ref`, `reward_experience`, `reward_gold`, and `reward_drop_vnums`
- preserving the descriptor across respawn does **not** mean rewards are automatically granted on respawn; the descriptor is only applied again after a later accepted killing hit in a fresh live loop
- dead/live HP state remains runtime-owned and separate from account or content persistence

## Combined reward descriptor coverage

The current runtime test coverage explicitly freezes the combined descriptor case as one kill-side transaction:
- one accepted killing hit may carry EXP, gold, and fixed drop-vnum entries together
- the self-visible frame order stays death/clear first, then EXP point-change, gold point-change, ground-add, and ownership
- the scalar EXP/gold account snapshot is saved before those scalar point-change frames are emitted
- the player runtime's scalar reward helper intentionally ignores `reward_drop_vnums` while applying EXP/gold, so combined descriptors are not rejected merely because a separate drop channel is present
- the drop is registered as a runtime ground item after the same accepted kill and remains non-persistent until pickup

This regression coverage matters because scalar persistence and drop registration use different runtime seams.
A combined descriptor must not accidentally suppress one reward family just because the other family is present.

## Success definition

The repository can now say:
- non-player rewards are a documented bootstrap seam rather than an implied future system
- default bootstrap combatants are still rewardless
- authored spawn groups may carry deterministic EXP, gold, and fixed drop-vnum descriptors
- a single accepted kill can emit EXP, gold, and owned drop feedback together in documented order
- accepted non-player death is preserved even when reward application fails
- scalar rewards persist before their point-change frames are emitted
- item drops become owned ground items and persist to inventory only through the normal pickup path
- timed respawn rebuild preserves authored reward descriptor metadata so later kills continue to use the same content contract

Broader reward, loot-table, party, quest, and level-up systems remain future work.
