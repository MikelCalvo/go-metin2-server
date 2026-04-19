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
- `server_public_key` is the server X25519 public key for this session.
- `challenge` is a fresh random 32-byte server challenge.
- `server_time` is compatibility data sent during the handshake and preserved on the wire.

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
- this packet carries the client X25519 public key plus the challenge answer material
- `challenge_response` is HMAC-SHA512/256 over the server challenge using the client->server session key derived from the X25519+BLAKE2b key exchange

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
- `encrypted_token` is an XChaCha20-Poly1305 ciphertext of a 32-byte session token using the server->client session key
- `nonce` is the 24-byte XChaCha20-Poly1305 nonce used for that one-time token encryption

## Working handshake flow

The current server-owned handshake flow is:

1. the TCP session starts in `HANDSHAKE`
2. the server generates a fresh X25519 keypair and challenge, then emits `KEY_CHALLENGE`
3. the client may emit `PONG` at any time during `HANDSHAKE`; it is accepted but does not advance the phase
4. the client emits `KEY_RESPONSE`
5. the server derives libsodium-compatible session keys from X25519 shared secret + BLAKE2b, then verifies the HMAC challenge response
6. if the response is accepted, the server emits plaintext `KEY_COMPLETE`
7. the server transitions the session to `LOGIN`
8. the server emits encrypted `PHASE(LOGIN)`
9. subsequent legacy traffic is encrypted with XChaCha20 stream mode using directional fixed nonces:
   - server -> client nonce prefix `0x01`
   - client -> server nonce prefix `0x02`

## Slice scope

This slice freezes the control-plane flow, packet layouts, and the current cryptographic contract for the secure legacy session bootstrap.

It does not yet freeze:
- long-term session-token semantics beyond "client must decrypt it successfully"
- auth-server-specific policy layered on top of the shared secure transport
- retry/backoff policy for failed handshakes
- end-to-end proof against every client build in the wild
