# Game Ping/Pong Control Path

This document freezes the first minimal control-plane `PING`/`PONG` behavior relevant once a session is already in `GAME`.

The goal of this slice is deliberately narrow:
- preserve the `PING` packet layout as server-owned control data
- accept a header-only client `PONG` while the session is in `GAME`
- keep the session in `GAME`
- emit no server response when `PONG` is received

This slice does not yet add a periodic ping scheduler.
It only freezes the packet codec and the accepted reply path.

## Covered packets

- `PING`
- `PONG`

## Envelope

The packets use the project-wide CG/GC frame envelope:

- `header` — `uint16`, little-endian
- `length` — `uint16`, little-endian, total frame size including the 4-byte envelope
- `payload` — packet-specific bytes

See `frame-layout.md` for the envelope contract.

## `PING`

Direction:
- server -> client

Header:
- `0x0007`

Payload layout:
- `server_time` — `uint32`, little-endian

Frame length:
- `8` bytes total (`4 + 4`)

Notes:
- `server_time` is control-plane compatibility data.
- The first implementation only freezes the codec; it does not schedule periodic probes yet.

## `PONG`

Direction:
- client -> server

Header:
- `0x0006`

Payload layout:
- none

Frame length:
- `4` bytes total (`4 + 0`)

Notes:
- `PONG` is a header-only control reply.
- The first implementation accepts it in `GAME` as a no-op.

## Working flow

The current server-owned behavior is:

1. the session is already in `GAME`
2. the server may emit `PING(server_time)`
3. the client may reply with `PONG`
4. the server accepts `PONG`
5. the server emits no response packet
6. the session remains in `GAME`

## Slice scope

This slice freezes:
- `PING` encode support
- `PONG` decode support
- tolerant `PONG` acceptance in `GAME`
- no-response behavior for `PONG`
- phase-stable handling in the live game session

It does not yet freeze:
- periodic ping cadence
- timeout or disconnect policy
- server-time synchronization semantics beyond the carried `server_time` field
- acceptance of `PONG` outside the currently documented phases
