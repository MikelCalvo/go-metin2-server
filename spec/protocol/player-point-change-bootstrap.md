# Player-point-change bootstrap

This document freezes the first minimal `PLAYER_POINT_CHANGE` behavior used by the bootstrap runtime after `ENTERGAME` and by the currently owned narrow gameplay point-update surfaces.

The goal of this slice is narrow:
- emit a deterministic self-only point refresh after the selected character enters `GAME`
- keep the bootstrap limited to one selected character
- avoid adding a broader stat system too early

It does not yet define a general-purpose stat recalculation engine beyond the currently owned template-backed item, non-player reward, and practice-mob retaliation point-change reuses.

## Covered packet

- `PLAYER_POINT_CHANGE` server -> client (`0x0215`)

## Working flow

The current project-owned bootstrap behavior after `ENTERGAME` is:

1. the session transitions to `GAME`
2. the server emits:
   - `PHASE(GAME)`
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
   - `PLAYER_POINT_CHANGE`
3. the `PLAYER_POINT_CHANGE` frame refreshes one deterministic point for the selected character only

This slice is intentionally narrow:
- only the selected character receives the point refresh
- only one deterministic point-change frame is emitted during bootstrap
- no fanout to other sessions is required yet
- no general-purpose stat recalculation engine is required yet

## Packet layout

Direction:
- server -> client

Header:
- `0x0215`

Payload layout:
- `vid` — `uint32`
- `type` — `uint8`
- `amount` — `int32`
- `value` — `int32`

Frame length:
- `17` bytes total (`4 + 13`)

## Current bootstrap behavior

The bootstrap runtime currently emits one deterministic self-only point refresh:
- `type = 1`
- `amount` mirrors the selected character bootstrap point value at index `1`
- `value` mirrors the same selected character bootstrap point value

For the current stub/bootstrap characters this means:
- existing selected character refresh uses its persisted point value
- newly created selected character refresh uses its initial point value

This is good enough for the first `PLAYER_POINT_CHANGE` slice, but not yet the final compatibility target.

The same packet is now also reused by the first owned `/use_item <slot>` vertical:
- runtime `type` comes from `item_template.use_effect.point_type`
- runtime `amount` comes from `item_template.use_effect.point_delta`
- runtime `value` mirrors the updated selected-character point at `item_template.use_effect.point_index`
- the current seeded bootstrap consumable template still resolves to `type = 1`, `amount = 50`, and `value = updated Points[1]`

The same packet is now also reused by the first narrow template-backed equip/unequip point slice:
- successful `/equip_item <from> <equip_slot>` and `/unequip_item <equip_slot> <to>` can append one self-only `PLAYER_POINT_CHANGE` when the matched item template carries `equip_effect` on that same authored `equip_slot`
- runtime `type` comes from `item_template.equip_effect.point_type`
- runtime `amount` comes from the authored signed `item_template.equip_effect.point_delta` on equip and its inverse on unequip; positive deltas act as bonuses and negative deltas act as penalties
- runtime `value` mirrors the updated selected-character point at `item_template.equip_effect.point_index`
- the current seeded bootstrap practice blade template still resolves to `vnum = 12200`, `type = 1`, `delta = +/-10`, and `value = updated Points[1]`
- the current seeded cursed practice blade template resolves to `vnum = 12201`, `type = 1`, `amount = -10` on equip and `+10` on unequip, and `value = updated Points[1]`

The same packet is now also reused by the first non-player reward seam:
- accepted killing hits against authored `spawn_groups` practice mobs can append self-only EXP and/or gold point changes after `DEAD` plus `TARGET(0, 0)`
- EXP uses the currently owned bootstrap experience point type and persists to the selected character before its point-change frame is emitted
- gold uses the currently owned bootstrap gold point type and persists to the selected character before its point-change frame is emitted
- deterministic item-drop rewards remain on the ground-item packet families instead of this point-change carrier

The same packet is also reused by the first content-practice-mob retaliation seam:
- accepted owner-side hits and the delayed server-origin retaliation cadence can append self-only HP point-loss updates to the engaged owner
- those retaliation point-loss updates are runtime-only today; they do not persist to the selected account snapshot until broader player-death/recovery policy is owned
- when retaliation reaches the current bootstrap zero-HP floor, the combat/death docs own the companion `DEAD(owner_vid)` and target-clear choreography

## Out of scope

This slice does not yet freeze:
- a general-purpose point-change stream beyond the explicitly documented item, reward, and retaliation surfaces above
- point-change updates for other entities
- derived stat recalculation rules beyond the first narrow template-backed equip/unequip point delta
- general-purpose multi-effect template execution
- buffs, level-up recalculation, reward sharing, or full combat damage/stat policy
