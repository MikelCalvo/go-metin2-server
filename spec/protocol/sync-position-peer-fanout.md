# Sync Position Peer Fanout

This document freezes the first minimal `SYNC_POSITION` fanout behavior for players that are already mutually visible in the bootstrap runtime.

The goal of this slice is narrow:
- keep the mover's current deterministic self `SYNC_POSITION` reply intact
- queue one `SYNC_POSITION` replication frame to already-visible peers
- avoid broadening the slice into range filtering, sectors, or richer world reconciliation

## Covered packet

- `SYNC_POSITION` server -> client (`0x0304`)

## Working flow

The current bootstrap runtime behavior is:

1. player A and player B are already visible to each other in `GAME`
2. player B sends `SYNC_POSITION`
3. player B receives the normal deterministic self `SYNC_POSITION` reply
4. player A receives one queued server-initiated `SYNC_POSITION` replication for player B
5. the queued replication reuses the same payload shape as the mover reply and carries player B's updated `vid`/position tuple

## Current scope

This slice freezes:
- queued peer `SYNC_POSITION` replication after visibility is already established
- reuse of the existing `SYNC_POSITION` server packet shape for peer fanout
- mover and peer seeing the same updated coordinates for that reconciliation event

It does not yet freeze:
- movement range filtering
- sector/interest management
- multi-actor sync batches beyond the currently accepted single-selected-character path
- late-join replay beyond the existing shared snapshot behavior
