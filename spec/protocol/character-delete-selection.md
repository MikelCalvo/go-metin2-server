# Character Deletion in Select

This document freezes the first minimal `CHARACTER_DELETE` slice on the selection surface.

The goal of this slice is narrow:
- accept `CHARACTER_DELETE` in `SELECT`
- delete one populated character slot deterministically
- emit a success frame that carries the cleared account slot index
- emit a minimal header-only failure frame when deletion is rejected
- persist the empty slot across fresh auth/game sessions

## Covered packets

- `CHARACTER_DELETE`
- `PLAYER_DELETE_SUCCESS`
- `PLAYER_DELETE_FAILURE`

## Envelope

All packets in this document use the project-wide CG/GC frame envelope:

- `header` — `uint16`, little-endian
- `length` — `uint16`, little-endian, total frame size including the 4-byte envelope
- `payload` — packet-specific bytes

See `frame-layout.md` for the envelope contract.

## `CHARACTER_DELETE`

Direction:
- client -> server

Header:
- `0x0202`

Payload layout:
- `index` — `uint8`
- `private_code` — fixed `8` bytes

Frame length:
- `13` bytes total (`4 + 1 + 8`)

String rules:
- the field is fixed-width
- the current slice expects a NUL-terminated code inside the 8-byte field
- effective maximum string length is `7` bytes in practice for this first implementation

## `PLAYER_DELETE_SUCCESS`

Direction:
- server -> client

Header:
- `0x020E`

Payload layout:
- `account_index` — `uint8`

Frame length:
- `5` bytes total (`4 + 1`)

## `PLAYER_DELETE_FAILURE`

Direction:
- server -> client

Header:
- `0x020F`

Payload layout:
- none

Frame length:
- `4` bytes total

Notes:
- the client family historically associates `0x020F` with a wrong-social-id delete failure path
- this project-owned minimal slice uses it as a generic header-only delete rejection placeholder
- stricter delete-policy semantics can be layered later without changing the success path

## Working flow

The current server-owned behavior is:

1. the session is already in `SELECT`
2. the client sends `CHARACTER_DELETE`
3. the server validates that the slot exists and is populated
4. the server rejects an empty private-code string in the first minimal implementation
5. on success, the server clears the slot and persists the updated account snapshot
6. the server emits `PLAYER_DELETE_SUCCESS(account_index)` and stays in `SELECT`
7. on rejection, the server emits `PLAYER_DELETE_FAILURE` and stays in `SELECT`

## Scope

This slice freezes:
- the delete request layout
- the success response layout
- the minimal failure response layout
- persistence of the cleared slot

It does not yet freeze:
- real social-id validation
- any rename/change-name interaction after delete
- account-wide safety policies beyond the minimal populated-slot check
