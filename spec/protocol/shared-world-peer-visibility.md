# Shared-World Peer Visibility

This document freezes the first minimal peer-visibility behavior across concurrent `gamed` sessions in the bootstrap runtime.

The goal of this slice is narrow:
- let a newly entering player see peers that are already connected
- let already-connected peers receive the newcomer as a queued server-initiated burst
- let already-connected peers receive `CHARACTER_DEL` when that peer disconnects

This slice is still intentionally minimal.
It does not yet fan out movement, chat, combat, or inventory changes.

## Covered packets

- `CHARACTER_ADD`
- `CHAR_ADDITIONAL_INFO`
- `CHARACTER_UPDATE`
- `CHARACTER_DEL`

## Working flow

The current bootstrap runtime behavior is:

1. player A enters `GAME`
2. player A receives only the existing self-bootstrap burst
3. player B later enters `GAME`
4. if player A and player B share the same bootstrap `MapIndex`, player B receives:
   - the normal self-bootstrap burst
   - one visibility burst for player A:
     - `CHARACTER_ADD`
     - `CHAR_ADDITIONAL_INFO`
     - `CHARACTER_UPDATE`
5. if player A and player B share the same bootstrap `MapIndex`, player A receives the same three peer-visibility frames for player B via the queued server-frame runtime hook
6. when player B disconnects, player A receives `CHARACTER_DEL` carrying player B's `vid` only if they shared the same bootstrap `MapIndex`

## `CHARACTER_DEL`

Direction:
- server -> client

Header:
- `0x0208`

Payload layout:
- `vid` — `uint32`, little-endian

Frame length:
- `8` bytes total (`4 + 4`)

Notes:
- the first implementation uses `CHARACTER_DEL` only for peer removal
- the payload is minimal and only identifies the leaving actor by `vid`

## Current scope limits

This slice freezes:
- peer snapshot bootstrap for players already connected on the same bootstrap `MapIndex` when a new player enters
- queued peer enter notifications for already-connected sessions on the same bootstrap `MapIndex`
- queued peer remove notifications on disconnect within the same bootstrap `MapIndex`
- reuse of the existing `CHARACTER_ADD` / `CHAR_ADDITIONAL_INFO` / `CHARACTER_UPDATE` payloads for visible peers

It does not yet freeze:
- movement fanout to other sessions
- sync-position fanout to other sessions
- NPC or item visibility
- range/sector culling
- reconnect or warp-time world migration semantics
