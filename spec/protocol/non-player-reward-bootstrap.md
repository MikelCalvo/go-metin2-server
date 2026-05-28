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

The descriptor is not character persistence by itself.
It becomes player state only after the already-accepted death edge tries to apply the reward through the selected live session.

Default bootstrap `training_dummy` and `practice_mob` profile defaults remain rewardless unless content or tests explicitly author a non-zero descriptor.

## Validation

Reward descriptors fail closed when:
- `reward_experience` or `reward_gold` exceeds the current signed 32-bit `PLAYER_POINT_CHANGE` carrier range
- any `reward_drop_vnums` entry is `0`
- a runtime-generated ground-item VID for a configured drop would be `0`
- multiple drops in the same descriptor would collide on the generated ground-item VID

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
- if the player runtime rejects the scalar reward or account persistence fails, the accepted death/clear frames remain, but scalar reward frames are omitted and live scalar state is rolled back to the pre-reward snapshot

## Drop rewards

Drop rewards reuse the existing bootstrap ground-item families:
- `ITEM_GROUND_ADD`
- `ITEM_OWNERSHIP`
- later pickup still uses the normal owned `ITEM_PICKUP` path, producing `ITEM_GROUND_DEL`, `ITEM_SET`, and `ITEM_GET` as appropriate

Current rules:
- each configured drop spawns at the killer's current position
- each drop has count `1`
- each drop is owned by the killer's character name
- item drops are runtime ground items first; they do not mutate persisted inventory until an explicit pickup succeeds
- replayed pickup of the same ground VID fails closed after the first successful pickup removes it

## Success definition

The repository can now say:
- non-player rewards are a documented bootstrap seam rather than an implied future system
- default bootstrap combatants are still rewardless
- authored spawn groups may carry deterministic EXP, gold, and fixed drop-vnum descriptors
- accepted non-player death is preserved even when reward application fails
- scalar rewards persist before their point-change frames are emitted
- item drops become owned ground items and persist to inventory only through the normal pickup path

Broader reward, loot-table, party, quest, and level-up systems remain future work.
