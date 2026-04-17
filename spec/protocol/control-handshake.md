# Control-plane handshake

This document freezes the packet layouts used by the initial control-plane handshake.

It does not yet define the full socket-level session choreography.
That higher-level flow will be locked by end-to-end tests later.
For now, this document only freezes packet identity, direction, and payload layout.

## Covered packets

- `PHASE`
- `PING`
- `PONG`
- `KEY_CHALLENGE`
- `KEY_RESPONSE`
- `KEY_COMPLETE`

`PHASE`, `PING`, and `PONG` are already covered by code and tests.
This document adds the key-exchange packet layouts needed to move the handshake forward.

## Envelope

All packets in this document use the project-wide CG/GC frame envelope:

- `header` — `uint16`, little-endian
- `length` — `uint16`, little-endian, total frame size including the 4-byte envelope
- `payload` — packet-specific bytes

See `frame-layout.md` for the envelope contract.

## Packet layouts

### `KEY_CHALLENGE`

Direction:
- server -> client

Header:
- `0x000B`

Payload layout:
- `server_public_key` — 32 bytes
- `challenge` — 32 bytes
- `server_time` — `uint32`, little-endian

Frame length:
- `72` bytes total (`4 + 32 + 32 + 4`)

Notes:
- `server_public_key` is the server key material exposed to the client for the handshake.
- `challenge` is the server-generated challenge blob the client must answer.
- `server_time` is compatibility data sent during the handshake and should be preserved exactly.

### `KEY_RESPONSE`

Direction:
- client -> server

Header:
- `0x000A`

Payload layout:
- `client_public_key` — 32 bytes
- `challenge_response` — 32 bytes

Frame length:
- `68` bytes total (`4 + 32 + 32`)

Notes:
- this packet carries the client contribution to the key exchange plus the challenge answer material
- the server session layer will validate it in a later slice

### `KEY_COMPLETE`

Direction:
- server -> client

Header:
- `0x000C`

Payload layout:
- `encrypted_token` — 48 bytes
- `nonce` — 24 bytes

Frame length:
- `76` bytes total (`4 + 48 + 24`)

Notes:
- `encrypted_token` is preserved here as opaque compatibility bytes
- cryptographic validation and token semantics will be handled in a later slice

## Working handshake intent

The current working handshake intent is:

1. session starts in `HANDSHAKE`
2. server sends control-plane packets required by the client
3. client answers `PONG` and `KEY_RESPONSE` as required
4. server finishes the key exchange and can transition the session to `LOGIN`

The exact socket-level sequence must be frozen by dedicated session tests before any auth handler depends on it.
