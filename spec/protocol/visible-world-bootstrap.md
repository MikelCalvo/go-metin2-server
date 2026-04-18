# Visible-world bootstrap

This document freezes the first minimal visible-world packets emitted after `ENTERGAME`.

The goal of this slice is narrow:
- transition from `LOADING` to `GAME`
- emit the first visible-world insert for the selected character
- keep the bootstrap deterministic and self-character only

It does not yet freeze the broader entity stream for NPCs, mobs, items, or other players.

## Covered packets

- `PHASE(GAME)`
- `CHARACTER_ADD`
- `CHAR_ADDITIONAL_INFO`
- `CHARACTER_UPDATE`
- `PLAYER_POINT_CHANGE`

## Working flow

The current project-owned world bootstrap after `ENTERGAME` is:

1. the session is in `LOADING`
2. the client sends `ENTERGAME`
3. the server transitions to `GAME`
4. the server emits:
   - `PHASE(GAME)`
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
   - `PLAYER_POINT_CHANGE`

This slice is intentionally narrow:
- only the selected character is announced
- only the selected character gets a deterministic self-update refresh
- only the selected character gets a deterministic self-only point refresh
- no item/NPC/mob bursts are required yet
- no fanout to other sessions is required yet

## Packet layouts

### `CHARACTER_ADD`

Direction:
- server -> client

Header:
- `0x0205`

Payload layout:
- `vid` — `uint32`
- `angle` — `float32`
- `x`, `y`, `z` — `int32` each
- `type` — `uint8`
- `race_num` — `uint16`
- `moving_speed` — `uint8`
- `attack_speed` — `uint8`
- `state_flag` — `uint8`
- `affect_flags` — `2 * uint32`

Frame length:
- `38` bytes total (`4 + 34`)

### `CHAR_ADDITIONAL_INFO`

Direction:
- server -> client

Header:
- `0x0207`

Payload layout:
- `vid` — `uint32`
- `name` — fixed `65` bytes (`CHARACTER_NAME_MAX_LEN + 1`)
- `parts` — `4 * uint16`
  - armor
  - weapon
  - head
  - hair
- `empire` — `uint8`
- `guild_id` — `uint32`
- `level` — `uint32`
- `alignment` — `int16`
- `pk_mode` — `uint8`
- `mount_vnum` — `uint32`

Frame length:
- `97` bytes total (`4 + 93`)

## Current bootstrap behavior

The bootstrap runtime currently uses deterministic values for the visible insert:
- a fixed self-insert type for player characters
- deterministic movement/attack speed values
- deterministic state bootstrap
- selected character coordinates, race, empire, level, visible parts, self-update state, and one self-only point refresh

This is good enough for the first visible-world slice, but not yet the final compatibility target.

## Out of scope

This slice does not yet freeze:
- `CHARACTER_ADD2`
- `CHARACTER_UPDATE2`
- visible-world packets for other entities
- item ground packets
- targeting/combat packets
