# Auth-server login

This document freezes the minimal auth-server packet set needed for a real client to obtain a login key.

The goal of this slice is narrow:
- accept `LOGIN3` in `AUTH`
- return either `LOGIN_FAILURE` or `AUTH_SUCCESS`
- make the resulting login key usable by `gamed` without a DB round-trip

It does not yet freeze channel list UX, server selection UX, or any auth-backend protocol.

## Covered packets

- `LOGIN3`
- `LOGIN_FAILURE`
- `AUTH_SUCCESS`

## Envelope

All packets in this document use the project-wide CG/GC frame envelope:

- `header` — `uint16`, little-endian
- `length` — `uint16`, little-endian, total frame size including the 4-byte envelope
- `payload` — packet-specific bytes

See `frame-layout.md` for the envelope contract.

## Working flow

The current project-owned auth flow is:

1. the TCP session starts in `HANDSHAKE`
2. the control-plane handshake completes
3. the server emits `PHASE(AUTH)`
4. the client sends `LOGIN3`
5. on bad credentials, the server emits `LOGIN_FAILURE`
6. on success, the server emits `AUTH_SUCCESS`
7. the client can then reconnect to `gamed` and send `LOGIN2`

This slice intentionally keeps the auth key contract simple:
- the auth result can be backed by deterministic in-process data for now
- no external DB is required
- duplicate-login policy can stay minimal until later

## Packet layouts

### `LOGIN3`

Direction:
- client -> server

Header:
- `0x0102`

Payload layout:
- `login` — fixed `31` bytes, null-terminated string space (`LOGIN_MAX_LEN + 1`)
- `password` — fixed `31` bytes, null-terminated string space (`PASSWD_MAX_LEN + 1` in the legacy reference shape)

Frame length:
- `66` bytes total (`4 + 31 + 31`)

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
- common compatibility strings include `NOID`, `WRONGPWD`, and `ALREADY`
- this slice only needs a stable subset of failure strings

### `AUTH_SUCCESS`

Direction:
- server -> client

Header:
- `0x0108`

Payload layout:
- `login_key` — `uint32`, little-endian
- `result` — `uint8`

Frame length:
- `9` bytes total (`4 + 4 + 1`)

Notes:
- `0x0108` is the working compatibility value used by this project
- `result=1` means the login key is valid and can be presented to `gamed`
- this packet is not the normal wrong-password response path; credential failures still use `LOGIN_FAILURE`
