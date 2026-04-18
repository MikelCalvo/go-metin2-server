# Move Peer Fanout

This document freezes the first minimal movement fanout behavior for players that are already mutually visible in the bootstrap runtime.

The goal of this slice is narrow:
- keep the mover's current deterministic self `MOVE` ack path intact
- queue one `MOVE` replication frame to already-visible peers
- avoid broadening the slice into sync-position fanout or interest management yet

## Covered packet

- `MOVE` server -> client (`0x0302`)

## Working flow

The current bootstrap runtime behavior is:

1. player A and player B are already visible to each other in `GAME`
2. player B sends `MOVE`
3. player B receives the normal deterministic self `MOVE` ack
4. player A receives one queued server-initiated `MOVE` replication for player B
5. the queued replication reuses the same payload shape as the mover ack and carries player B's `vid`

## Current scope

This slice freezes:
- queued peer move replication after visibility is already established
- reuse of the existing `MOVE` server packet shape for peer fanout
- mover and peer seeing the same updated coordinates for that move event

It does not yet freeze:
- `SYNC_POSITION` fanout
- movement range filtering
- late-join replay beyond the existing shared snapshot behavior
- interpolation or timing policy beyond the currently carried wire fields
