# Sync-position bootstrap

This document freezes the first minimal `SYNC_POSITION` behavior used by the bootstrap runtime.

The goal of this slice is narrow:
- accept client-side position reconciliation after `ENTERGAME`
- keep the session stable when the client emits self-position sync frames
- answer with a deterministic server-side sync payload for the selected character only

It does not yet define broader multi-entity world reconciliation.

## Covered packets

- `SYNC_POSITION` client -> server (`0x0303`)
- `SYNC_POSITION` server -> client (`0x0304`)

## Working flow

The current project-owned bootstrap behavior in `GAME` is:

1. the session is already in `GAME`
2. the client sends `SYNC_POSITION`
3. the server filters the payload to the selected character VID
4. the server updates the selected character coordinates in the bootstrap runtime
5. the server emits one deterministic `SYNC_POSITION` response with that selected character position

This slice is intentionally narrow:
- only the selected character is reconciled
- extra VIDs in the client packet are ignored unless they match the selected character
- no fanout to other sessions is required yet
- no broad world-state authority model is frozen yet

## Packet layout

Each payload is a flat array of fixed-size elements.

### Element layout

- `vid` — `uint32`
- `x` — `int32`
- `y` — `int32`

Element size:
- `12` bytes

### `SYNC_POSITION` client -> server

Direction:
- client -> server

Header:
- `0x0303`

Payload:
- zero or more sync-position elements
- payload length must be a multiple of `12`

### `SYNC_POSITION` server -> client

Direction:
- server -> client

Header:
- `0x0304`

Payload:
- zero or more sync-position elements
- current bootstrap runtime emits exactly one element for the selected character when reconciliation is accepted

## Current bootstrap behavior

The bootstrap runtime currently uses a deterministic self-only reconciliation path:
- the selected character VID is the only authoritative target
- the accepted coordinates become the selected character coordinates for the running session
- the server answers with a one-element `SYNC_POSITION` payload reflecting the selected character position

This is good enough for the first sync-position slice, but not yet the final compatibility target.

## Out of scope

This slice does not yet freeze:
- multi-entity reconciliation rules
- distance validation or anti-cheat rules
- fanout to other clients
- map-sector visibility logic
- server-authoritative correction bursts for nearby entities
