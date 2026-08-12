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
8. that same `login_key` must remain reusable across the real client's pre-game reconnects (for example the second game socket opened by direct-enter on the selected character)

This slice intentionally keeps the auth key contract simple:
- the auth result can be backed by deterministic in-process data for now
- no external DB is required
- duplicate-login policy can stay minimal until later

## File-backed ticket integrity

When `authd` issues a one-shot login ticket for `gamed`, the durable JSON ticket must contain a non-zero `issued_at` timestamp. The store fills that field at issue time if the caller omits it, but already-committed ticket files with a missing or zero `issued_at` are invalid and fail closed during load, validation, stale-ticket preview, stale-ticket cleanup, and consume paths. That makes `/local/login-tickets/issued-before/preview` and `/local/login-tickets/issued-before/cleanup` safe to use as age-based recovery primitives: manually assembled or partially migrated ticket snapshots cannot avoid stale-ticket policy by silently decoding to Go's zero time.

## Schema-only migration boundary

The project-owned migration catalog now includes `0007_auth_login_ticket_handoff` for the authd-to-gamed handoff state. It is a schema/backfill contract only; the shipped runtime still uses the JSON `internal/loginticket` store.

The schema records active non-zero login keys, issued timestamps, login/original-normalized login, empire context, optional consumed timestamp, and a transitional character snapshot JSON payload. A partial unique index keeps active `login_key` rows unique while allowing future historical/consumed rows. This does not add a DB-backed ticket repository, apply/rollback command, or SQL consume implementation yet.

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
