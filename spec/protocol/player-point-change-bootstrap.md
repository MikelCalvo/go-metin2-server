# Player-point-change bootstrap

This document freezes the first minimal `PLAYER_POINT_CHANGE` behavior used by the bootstrap runtime after `ENTERGAME`.

The goal of this slice is narrow:
- emit a deterministic self-only point refresh after the selected character enters `GAME`
- keep the bootstrap limited to one selected character
- avoid adding a broader stat system too early

It does not yet define general-purpose point updates during gameplay.

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

## Out of scope

This slice does not yet freeze:
- repeated point-change streams during gameplay
- point-change updates for other entities
- derived stat recalculation rules
- inventory, buffs, or combat-driven point updates
