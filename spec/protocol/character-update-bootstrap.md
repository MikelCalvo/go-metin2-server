# Character-update bootstrap

This document freezes the first minimal `CHARACTER_UPDATE` behavior used by the bootstrap runtime after `ENTERGAME`.

The goal of this slice is narrow:
- emit a deterministic self-only state refresh after the selected character is inserted into the visible world
- keep the bootstrap limited to one selected character
- avoid pulling in broader world-state systems too early

It does not yet define update fanout for other entities.

## Covered packet

- `CHARACTER_UPDATE` server -> client (`0x0209`)

## Working flow

The current project-owned bootstrap behavior after `ENTERGAME` is:

1. the session transitions to `GAME`
2. the server emits:
   - `PHASE(GAME)`
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
3. the `CHARACTER_UPDATE` frame refreshes the selected character state with deterministic self-only values

This slice is intentionally narrow:
- only the selected character is refreshed
- no fanout to other sessions is required yet
- no `CHARACTER_UPDATE2` is required yet
- no dynamic state machine for mounts, affects, or PK mode is required yet

## Packet layout

Direction:
- server -> client

Header:
- `0x0209`

Payload layout:
- `vid` — `uint32`
- `parts` — `4 * uint16`
  - armor
  - weapon
  - head
  - hair
- `moving_speed` — `uint8`
- `attack_speed` — `uint8`
- `state_flag` — `uint8`
- `affect_flags` — `2 * uint32`
- `guild_id` — `uint32`
- `alignment` — `int16`
- `pk_mode` — `uint8`
- `mount_vnum` — `uint32`

Frame length:
- `38` bytes total (`4 + 34`)

## Current bootstrap behavior

The bootstrap runtime currently uses deterministic values for the selected-character refresh:
- visible parts derived from the selected character snapshot
- deterministic movement and attack speed values
- deterministic state flag and affect flags
- selected character guild, alignment, pk mode, and mount bootstrap values

This is good enough for the first `CHARACTER_UPDATE` slice, but not yet the final compatibility target.

## Out of scope

This slice does not yet freeze:
- `CHARACTER_UPDATE2`
- update fanout to nearby players
- dynamic affect/mount transitions
- broader visible-world state changes beyond the selected character
