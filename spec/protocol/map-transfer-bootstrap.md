# Map-Transfer Bootstrap Self-Session Contract

This document freezes the first owned self-session reply contract for bootstrap map transfer.

It sits on top of:
- `map-relocation-visibility-rebuild.md`
- `bootstrap-map-transfer-contract.md`
- `exact-position-bootstrap-transfer-trigger.md`

Those documents already freeze the server-side relocation primitive, the structured preview/commit result, and the first gameplay-side trigger.
What this document adds is the current honest answer to a narrower question:

**What does the moved player itself receive on the game socket today when a bootstrap transfer is committed?**

## Scope

This contract applies only to:
- an already-connected selected bootstrap player in `GAME`
- a transfer that is committed through the current bootstrap runtime
- transfers that move the player from one effective `MapIndex` visibility scope to another

It does not yet freeze a final gameplay warp/loading UX.

## Current owned self-session contract

When a bootstrap transfer is committed from gameplay today:

1. the normal same-map gameplay reply is suppressed
2. there is no immediate self-visible warp packet
3. the moved session instead receives queued self visibility-delta frames
4. those queued frames reflect the same visibility rebuild that the runtime applies to peers

## Suppressed immediate replies

For the current bootstrap runtime:

- a matched transfer-trigger `MOVE` produces **no immediate self `MOVE_ACK`**
- a matched transfer-trigger `SYNC_POSITION` produces **no immediate self `SYNC_POSITION_ACK`**

This is intentional.
The current runtime treats the map transfer as a visibility-rebuild event, not as a finished client-visible warp choreography.

## Current self visibility-delta shape

When the transfer changes effective map visibility, the moved session currently receives queued frames in this shape:

1. one `CHARACTER_DEL` for each peer that stops being visible from the source map scope
2. then the normal destination-map visibility burst for each peer that becomes visible:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`

This reuses the already-owned visibility packet family.
No transfer-specific self packet is frozen yet.

## Example: move from map 1 to map 42

If:
- player A stays visible on source map `1`
- player B is the moved selected character
- player C is already visible on destination map `42`

then a successful committed transfer currently produces this self-session result for player B:

1. no immediate `MOVE_ACK` / `SYNC_POSITION_ACK`
2. queued `CHARACTER_DEL` for player A
3. queued destination visibility burst for player C:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`

At the same time:
- player A receives `CHARACTER_DEL` for player B
- player C receives the destination visibility burst for player B using B's relocated snapshot

## Same-map transfers

If the runtime updates position but keeps the player on the same effective map scope, this self-session transfer contract does not emit a visibility rebuild.
That remains a normal same-map movement/sync concern rather than a visible-world transfer concern.

## Why this slice exists

The project already owns:
- the structured server-side transfer commit/preview result
- the visibility rebuild primitive underneath it
- the exact-position gameplay trigger that can invoke transfer

But without this document, the moved player's own wire-visible result still lived only as an implication of tests and implementation.

This slice makes the current behavior explicit without overstating progress toward a final warp system.

## Explicit non-goals

This slice still does not freeze:
- a dedicated client-originated warp request packet
- a final self-visible loading-screen or warp packet
- inter-channel migration
- reconnect/resume semantics across transfer
- portal/script-driven public content loading
- AOI/range/sector visibility rules beyond the current bootstrap topology
- non-player entity transfer
