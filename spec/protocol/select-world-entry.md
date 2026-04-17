# Selection to world entry

This document freezes the minimal packet and phase choreography needed to move from the selection surface into the live world.

The goal of this slice is narrow:
- accept `CHARACTER_CREATE` in `SELECT`
- return `PLAYER_CREATE_SUCCESS` or `PLAYER_CREATE_FAILURE`
- accept `EMPIRE` selection in `SELECT` when the account is empty
- accept `CHARACTER_SELECT`
- enter `LOADING`
- send the minimum bootstrap packets
- accept `ENTERGAME`
- transition into `GAME`

It does not yet freeze the full visible-world packet set.

## Covered packets

- `CHARACTER_CREATE`
- `PLAYER_CREATE_SUCCESS`
- `PLAYER_CREATE_FAILURE`
- `CHARACTER_SELECT`
- `ENTERGAME`
- `MAIN_CHARACTER`
- `PLAYER_POINTS`
- `CHARACTER_ADD`
- `CHAR_ADDITIONAL_INFO`

## Envelope

All packets in this document use the project-wide CG/GC frame envelope:

- `header` — `uint16`, little-endian
- `length` — `uint16`, little-endian, total frame size including the 4-byte envelope
- `payload` — packet-specific bytes

See `frame-layout.md` for the envelope contract.

## Working flow

The current project-owned selection/world-entry flow is:

1. the session is in `SELECT`
2. if the account is empty and has no chosen empire yet, the client may send `EMPIRE` selection (`0x010A`)
3. on accepted empire selection, the server emits `EMPIRE` and stays in `SELECT`
4. the client may send `CHARACTER_CREATE`
5. on create success, the server emits `PLAYER_CREATE_SUCCESS` and stays in `SELECT`
6. on create failure, the server emits `PLAYER_CREATE_FAILURE` and stays in `SELECT`
7. the client sends `CHARACTER_SELECT`
8. the server validates the slot and transitions to `LOADING`
9. the server emits:
   - `PHASE(LOADING)`
   - `MAIN_CHARACTER`
   - `PLAYER_POINTS`
10. the client sends `ENTERGAME`
11. the server transitions to `GAME`
12. the server emits:
   - `PHASE(GAME)`
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`

This slice keeps the bootstrap minimal on purpose:
- no item stream is required yet
- no quickslot stream is required yet
- no world-entity burst is required yet
- those can be layered in later slices once a client can cross the boundary reliably

## Packet layouts

### `CHARACTER_CREATE`

Direction:
- client -> server

Header:
- `0x0201`

Payload layout:
- `index` — `uint8`
- `name` — fixed `65` bytes (`CHARACTER_NAME_MAX_LEN + 1`)
- `race_num` — `uint16`
- `shape` — `uint8`
- `con`, `int`, `str`, `dex` — `uint8` each

Frame length:
- `76` bytes total (`4 + 72`)

### `PLAYER_CREATE_SUCCESS`

Direction:
- server -> client

Header:
- `0x020C`

Payload layout:
- `index` — `uint8`
- `player` — packed `SimplePlayer` record (`103` bytes)

Frame length:
- `108` bytes total (`4 + 1 + 103`)

### `PLAYER_CREATE_FAILURE`

Direction:
- server -> client

Header:
- `0x020D`

Payload layout:
- `type` — `uint8`

Frame length:
- `5` bytes total (`4 + 1`)

### `CHARACTER_SELECT`

Direction:
- client -> server

Header:
- `0x0203`

Payload layout:
- `index` — `uint8`

Frame length:
- `5` bytes total (`4 + 1`)

### `ENTERGAME`

Direction:
- client -> server

Header:
- `0x0204`

Payload layout:
- none

Frame length:
- `4` bytes total

### `MAIN_CHARACTER`

Direction:
- server -> client

Header:
- `0x0210`

Payload layout:
- `vid` — `uint32`
- `race_num` — `uint16`
- `name` — fixed `65` bytes (`CHARACTER_NAME_MAX_LEN + 1`)
- `bgm_name` — fixed `25` bytes (`24 + 1`)
- `bgm_volume` — `float32`, little-endian IEEE-754
- `x`, `y`, `z` — `int32` each
- `empire` — `uint8`
- `skill_group` — `uint8`

Frame length:
- `118` bytes total (`4 + 114`)

### `PLAYER_POINTS`

Direction:
- server -> client

Header:
- `0x0214`

Payload layout:
- `points` — `255 * int32` (`POINT_MAX_NUM`)

Frame length:
- `1024` bytes total (`4 + 255*4`)

## Slice scope

This slice freezes the minimal `SELECT -> LOADING -> GAME` boundary, including in-phase character creation.

It does not yet freeze:
- quickslot bootstrap
- skill-level bootstrap
- item bootstrap
- visible-world insert packets for other entities after `ENTERGAME`
- time/channel/world metadata packets
