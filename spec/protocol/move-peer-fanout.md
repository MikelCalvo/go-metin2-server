# Move Peer Fanout

This document freezes the first minimal movement fanout behavior for players that are already mutually visible in the bootstrap runtime, plus the first AOI-aware visibility rebuild that can now happen on `MOVE` when the runtime is configured with the opt-in radius policy.

The goal of this slice is narrow:
- keep the mover's current deterministic self `MOVE` ack path intact
- queue one `MOVE` replication frame to already-visible peers
- when configured AOI causes the visible-peer set to change on `MOVE`, rebuild that visibility with add/delete frames instead of pretending the peer set stayed constant
- when configured AOI causes already-seeded static actors to enter or leave the mover's visible world on `MOVE`, queue the corresponding self-facing bootstrap/delete frames too
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

There is now one more content-facing AOI branch for already-seeded static actors:

1. player A is in `GAME` and the runtime already owns one or more static actors on that map
2. player A sends `MOVE` and crosses the configured AOI boundary relative to one of those static actors
3. player A still receives the normal deterministic self `MOVE` ack
4. if the move makes a static actor newly visible, player A receives the normal actor bootstrap burst (`CHARACTER_ADD`, `CHAR_ADDITIONAL_INFO`, `CHARACTER_UPDATE`) via queued server frames
5. if the move makes a previously visible static actor leave range, player A receives the corresponding `CHARACTER_DEL` teardown via queued server frames

## Current scope

This slice freezes:
- queued peer move replication after same-map visibility is already established
- reuse of the existing `MOVE` server packet shape for peer fanout to peers that stay visible across the move
- mover and peer seeing the same updated coordinates for that move event
- AOI-aware add/delete visibility rebuild on `MOVE` when the configured runtime visibility policy changes the peer set during the move
- AOI-aware self-facing add/delete rebuild for already-seeded static actors when `MOVE` changes whether they share visible world with the mover

It does not yet freeze:
- sync-position AOI visibility rebuild
- interpolation or timing policy beyond the currently carried wire fields
- continuous sector streaming beyond the currently computed before/after visibility sets
