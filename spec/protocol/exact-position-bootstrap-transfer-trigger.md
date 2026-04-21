# Exact-Position Bootstrap Transfer Trigger

This document freezes the first gameplay-side trigger that can commit the bootstrap map-transfer contract.

It intentionally does not freeze the final client-visible warp choreography.
The goal of this slice is smaller: define when gameplay can request a transfer, what runtime state changes happen, and what is explicitly still deferred.

## Scope

This trigger applies only to:

- an already-connected selected character in `GAME`
- the current bootstrap runtime
- transfer targets that are already representable by `TransferCharacter(name, map_index, x, y)`

This trigger sits on top of `bootstrap-map-transfer-contract.md`.
It does not replace that document; it defines the first gameplay path that is allowed to invoke the commit side of that contract.

## Trigger shape

The first frozen gameplay trigger is an exact-position trigger with this internal shape:

- `source_map_index`
- `source_x`
- `source_y`
- `target_map_index`
- `target_x`
- `target_y`

A trigger matches only when all of the following are true:

1. the selected character's current effective `MapIndex` equals `source_map_index`
2. the candidate gameplay position equals `source_x` and `source_y` exactly
3. `target_map_index` is non-zero

If multiple triggers would match, the first configured trigger wins.

## Packet sources

The current gameplay trigger is evaluated from two already-owned packet paths:

1. `MOVE`
   - the candidate position is the incoming `MOVE` packet `x` / `y`

2. `SYNC_POSITION`
   - only the element whose `VID` matches the selected character is eligible
   - the candidate position is that element's `x` / `y`

No new client packet type is frozen here.

## Match behavior

When an exact-position trigger matches:

1. the normal same-map position update path is skipped
2. the runtime invokes the bootstrap transfer commit path with the trigger target
3. bootstrap account snapshot persistence must succeed before the transfer is committed
4. shared-world visibility is rebuilt by the existing transfer contract
5. future peer-scoped movement and chat fanout follow the destination `MapIndex`

## Current self-session reply contract

The current gameplay trigger deliberately does not freeze a final self-visible warp packet yet.

The owned self-session transfer reply contract now lives in:
- `map-transfer-bootstrap.md`

That document freezes the current bootstrap behavior:
- no immediate self `MOVE_ACK`
- no immediate self `SYNC_POSITION_ACK`
- queued self visibility-delta frames when peers leave or enter the moved character's visible world

This is intentional.
The final self-facing loading / warp reply remains a future protocol slice.

## Non-match behavior

When no trigger matches, existing behavior is unchanged:

- `MOVE` continues through the normal same-map position update path
- `SYNC_POSITION` continues through the normal same-map synchronization path

## Failure behavior

If a trigger matches but the transfer commit cannot be applied:

- the gameplay packet is currently treated as rejected for this bootstrap runtime
- no transfer mutation is committed
- no peer visibility changes are queued
- no map-occupancy change is committed

## Explicit non-goals

This slice still does not freeze:

- the final client-visible map-loading choreography
- any dedicated client-originated warp request packet
- portal/NPC/script data loading from a public content format
- inter-channel migration
- reconnect/resume semantics across transfer
- generic entity transfer beyond the selected bootstrap player
