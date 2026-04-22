# Transfer Rebootstrap Burst

This document freezes the first owned self-session rebootstrap burst used after a successful bootstrap map transfer.

It sits on top of:
- `bootstrap-map-transfer-contract.md`
- `loading-to-game-bootstrap-burst.md`
- `exact-position-bootstrap-transfer-trigger.md`

Those documents already freeze the structured transfer commit path, the normal enter-game bootstrap burst, and the gameplay-side trigger.
What this document adds is the first explicit answer to a narrower question:

**What does the moved player itself receive on the same game socket after a successful bootstrap transfer commit?**

## Scope

This contract applies only to:
- an already-connected selected bootstrap player in `GAME`
- a transfer that is committed through the current bootstrap runtime
- transfers that move the player from one effective `MapIndex` visibility scope to another
- the current single-process bootstrap runtime on the same encrypted game socket

It does not yet freeze a final warp/loading-screen UX, reconnect choreography, or inter-channel migration.

## Current owned self-session contract

When a bootstrap transfer is committed from gameplay today:

1. the normal same-map self reply is suppressed
   - no immediate self `MOVE_ACK`
   - no immediate self `SYNC_POSITION_ACK`
2. the moved session instead receives an immediate **transfer rebootstrap burst** on that same game socket
3. the burst reuses the owned selected-character bootstrap packet family with the relocated snapshot
4. trailing peer visibility deltas are appended after that self burst

## Self rebootstrap frames

The moved player first receives the relocated selected-character burst in this exact order:

1. `CHARACTER_ADD`
2. `CHAR_ADDITIONAL_INFO`
3. `CHARACTER_UPDATE`
4. `PLAYER_POINT_CHANGE`

These four frames reuse the same owned self-bootstrap family documented by `loading-to-game-bootstrap-burst.md`, but they are rebuilt from the **post-transfer** character snapshot.

## Trailing peer frames

After the relocated self burst, the moved player currently receives the transfer visibility deltas in this order:

1. one `CHARACTER_DEL` for each peer that stops being visible from the source map scope
2. then the normal destination-map visibility burst for each peer that becomes visible:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`

This keeps the moved player's own actor bootstrap deterministic while still reusing the existing peer-visibility packet family.

## Example: move from map 1 to map 42

If:
- player A stays visible on source map `1`
- player B is the moved selected character
- player C is already visible on destination map `42`

then a successful committed transfer currently produces this self-session result for player B:

1. relocated self burst for B:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`
   - `PLAYER_POINT_CHANGE`
2. `CHARACTER_DEL` for player A
3. destination visibility burst for player C:
   - `CHARACTER_ADD`
   - `CHAR_ADDITIONAL_INFO`
   - `CHARACTER_UPDATE`

At the same time:
- player A receives `CHARACTER_DEL` for player B
- player C receives the destination visibility burst for player B using B's relocated snapshot

## Same-map transfers

If the runtime updates position but keeps the player on the same effective map scope, this self-session transfer contract does not emit a rebootstrap burst.
That remains a normal same-map movement/sync concern rather than a visible-world transfer concern.

## Why this slice exists

The project now owns:
- the structured transfer commit/preview result
- the shared-world visibility rebuild underneath transfer
- the session-directory routing for live transport hooks
- a reusable selected-character bootstrap builder in `internal/worldentry`

Without this document, the moved player's transfer-visible result would still be split across test expectations and implementation details.

This slice makes the current behavior explicit without overstating progress toward a final warp system.

## Explicit non-goals

This slice still does not freeze:
- a dedicated client-originated warp request packet
- a final self-visible loading-screen or warp packet
- `PHASE(LOADING)` / `PHASE(GAME)` replay during transfer
- inter-channel migration
- reconnect/resume semantics across transfer
- portal/script-driven public content loading
- AOI/range/sector visibility beyond the current bootstrap topology
- non-player entity transfer
