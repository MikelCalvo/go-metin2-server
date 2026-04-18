# Client-Version Metadata During Loading

This document freezes the first minimal `CLIENT_VERSION` slice for the TMP4 boot path.

The goal of this slice is narrow:
- accept `CLIENT_VERSION` in `LOADING`
- decode its fixed-width metadata payload
- keep the session in `LOADING`
- emit no server response

This slice is intentionally tolerant.
It does not yet enforce any version policy.

## Covered packet

- `CLIENT_VERSION`

## Envelope

The packet uses the project-wide CG/GC frame envelope:

- `header` — `uint16`, little-endian
- `length` — `uint16`, little-endian, total frame size including the 4-byte envelope
- `payload` — packet-specific bytes

See `frame-layout.md` for the envelope contract.

## `CLIENT_VERSION`

Direction:
- client -> server

Header:
- `0x000D`

Payload layout:
- `executable_name` — fixed `33` bytes (`32 + 1`)
- `timestamp` — fixed `33` bytes (`32 + 1`)

Frame length:
- `70` bytes total (`4 + 33 + 33`)

String rules:
- strings are carried in fixed-width fields
- a trailing zero byte may terminate the string early
- unused bytes remain zero-filled
- the server trims at the first zero byte when decoding

## Working flow

The current server-owned behavior is:

1. the session is already in `LOADING`
2. the client may send `CLIENT_VERSION`
3. the server decodes `executable_name` and `timestamp`
4. the server emits no response packet
5. the session remains in `LOADING`
6. the client may still proceed to `ENTERGAME`

## Notes

- This packet is currently treated as metadata only.
- The first implementation does not persist the values.
- The first implementation does not reject by filename or timestamp.
- Version gating can be layered in later once compatibility evidence is stronger.

## Slice scope

This slice freezes:
- packet identity
- payload layout
- tolerant acceptance in `LOADING`
- no-response behavior

It does not yet freeze:
- any exact build string policy
- auth/login-time handling
- repeated-client-version policy beyond "accept and ignore"
- logging requirements
