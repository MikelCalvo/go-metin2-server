# Login and selection surface

This document freezes the first minimal packet set used after the control-plane handshake.

The goal of this slice is narrow:
- accept `LOGIN2` in `LOGIN`
- return a deterministic success or failure path
- reach the selection surface in `SELECT`
- allow empty accounts to choose an empire before character creation

It does not yet freeze full multi-step account setup beyond that bootstrap path.

## Covered packets

- `LOGIN2`
- `LOGIN_FAILURE`
- `EMPIRE`
- `EMPIRE` selection request
- `LOGIN_SUCCESS4`

## Envelope

All packets in this document use the project-wide CG/GC frame envelope:

- `header` — `uint16`, little-endian
- `length` — `uint16`, little-endian, total frame size including the 4-byte envelope
- `payload` — packet-specific bytes

See `frame-layout.md` for the envelope contract.

## Working flow

The current project-owned login flow is:

1. the session enters `LOGIN` after the control-plane handshake
2. the client sends `LOGIN2`
3. on failure, the server emits `LOGIN_FAILURE` and the session stays in `LOGIN`
4. on success, the server emits:
   - `EMPIRE`
   - `PHASE(SELECT)`
   - `LOGIN_SUCCESS4`
5. the session transitions to `SELECT`
6. if the account has no chosen empire and no characters yet, the client may send `EMPIRE` selection (`0x010A`)
7. on accepted empire selection, the server emits `EMPIRE` with the chosen value and stays in `SELECT`

This is intentionally narrower than the full legacy stack:
- no DB round-trip is required in this slice
- no auth-server split is required in this slice
- `LOGIN_KEY` is not part of the minimal happy path frozen here

## Packet layouts

### `LOGIN2`

Direction:
- client -> server

Header:
- `0x0101`

Payload layout:
- `login` — fixed `31` bytes, null-terminated string space (`LOGIN_MAX_LEN + 1`)
- `login_key` — `uint32`, little-endian

Frame length:
- `39` bytes total (`4 + 31 + 4`)

Notes:
- the login string should be treated as a fixed-size byte field on the wire
- project code may trim trailing null bytes when decoding

### `LOGIN_FAILURE`

Direction:
- server -> client

Header:
- `0x0106`

Payload layout:
- `status` — fixed `9` bytes, null-terminated string space (`ACCOUNT_STATUS_MAX_LEN + 1`)

Frame length:
- `13` bytes total (`4 + 9`)

Notes:
- this slice preserves the fixed-width status field exactly
- status values remain compatibility strings such as `FULL`, `SHUTDOWN`, or project-owned failure markers

### `EMPIRE`

Direction:
- server -> client

Header:
- `0x0109`

Payload layout:
- `empire` — `uint8`

Frame length:
- `5` bytes total (`4 + 1`)

### `EMPIRE` selection request

Direction:
- client -> server

Header:
- `0x010A`

Payload layout:
- `empire` — `uint8`

Frame length:
- `5` bytes total (`4 + 1`)

Notes:
- this minimal slice only accepts values `1..3`
- it is only meaningful for empty-account bootstrap flows

### `LOGIN_SUCCESS4`

Direction:
- server -> client

Header:
- `0x0105`

Payload layout:
- `players` — `4` packed `SimplePlayer` records
- `guild_ids` — `4 * uint32`
- `guild_names` — `4` fixed `13` byte strings (`GUILD_NAME_MAX_LEN + 1`)
- `handle` — `uint32`, little-endian
- `random_key` — `uint32`, little-endian

Frame length:
- `492` bytes total

## `SimplePlayer` wire layout

Each packed `SimplePlayer` record is `103` bytes and contains:
- `id` — `uint32`
- `name` — fixed `65` bytes (`CHARACTER_NAME_MAX_LEN + 1`)
- `job` — `uint8`
- `level` — `uint8`
- `play_minutes` — `uint32`
- `st`, `ht`, `dx`, `iq` — `uint8` each
- `main_part` — `uint16`
- `change_name` — `uint8`
- `hair_part` — `uint16`
- `dummy` — `4` raw bytes
- `x`, `y` — `int32`
- `addr` — `uint32`
- `port` — `uint16`
- `skill_group` — `uint8`

## Slice scope

This slice freezes only the minimal `LOGIN -> SELECT` behavior.

It does not yet freeze:
- `LOGIN_SUCCESS3`
- `LOGIN_KEY`
- `EMPIRE` selection request from the client
- character creation
- character selection
- loading/bootstrap
