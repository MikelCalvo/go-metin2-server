# Move Peer Fanout

This document freezes the first minimal movement fanout behavior for players that are already mutually visible in the bootstrap runtime, plus the first AOI-aware visibility rebuild that can now happen on `MOVE` when the runtime is configured with the opt-in radius policy.

The goal of this slice is narrow:
- keep the mover's current deterministic self `MOVE` ack path intact
- queue one `MOVE` replication frame to already-visible peers
- when configured AOI causes the visible-peer set to change on `MOVE`, rebuild that visibility with add/delete frames instead of pretending the peer set stayed constant
- avoid broadening the slice into sync-position fanout or full continuous interest-management yet

## Covered packet

- `MOVE` server -> client (`0x0302`)

## Working flow

The current bootstrap runtime behavior is:

1. player A and player B are already visible to each other in `GAME` because they share the same bootstrap `MapIndex`
2. player B sends `MOVE`
3. player B receives the normal deterministic self `MOVE` ack
4. player A receives one queued server-initiated `MOVE` replication for player B
5. the queued replication reuses the same payload shape as the mover ack and carries player B's `vid`

When the runtime is configured with opt-in radius AOI, there is now one more owned branch:

1. player A and player B are on the same effective map but are **not** currently visible because they are outside the configured radius
2. player B sends `MOVE` and crosses into A's visible range
3. player B still receives the normal deterministic self `MOVE` ack
4. player A receives the normal peer-entry burst for player B (`CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`) rather than a lone `MOVE` replication for an actor it had not seen yet
5. player B receives the symmetric peer-entry burst for player A via queued server frames
6. if player B later moves back out of range, both sides receive the corresponding `CHARACTER_DEL` teardown instead of silent disappearance

## Current scope

This slice freezes:
- queued peer move replication after same-map visibility is already established
- reuse of the existing `MOVE` server packet shape for peer fanout to peers that stay visible across the move
- mover and peer seeing the same updated coordinates for that move event
- AOI-aware add/delete visibility rebuild on `MOVE` when the configured runtime visibility policy changes the peer set during the move

It does not yet freeze:
- sync-position AOI visibility rebuild
- interpolation or timing policy beyond the currently carried wire fields
- continuous sector streaming beyond the currently computed before/after visibility sets
