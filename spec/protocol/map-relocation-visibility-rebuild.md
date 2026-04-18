# Map-Relocation Visibility Rebuild

This document freezes the first server-side visibility rebuild primitive for a connected player that is relocated from one bootstrap `MapIndex` to another.

This is intentionally narrower than a full warp flow.
The runtime reuses the existing peer-visibility packets to rebuild the visible player set, but it does not yet freeze any client-originated warp packet, loading-screen choreography, or full world transfer semantics.
The primitive exists in the shared-world runtime internals and is not yet wired into a client-facing warp flow.

## Covered packets

The runtime currently reuses only the already-owned visibility packets:

- `CHARACTER_ADD`
- `CHAR_ADDITIONAL_INFO`
- `CHARACTER_UPDATE`
- `CHARACTER_DEL`

## Working flow

When the current relocation primitive is invoked, it behaves as follows:

1. player A and player B are already connected and visible on bootstrap `MapIndex = 1`
2. player C is already connected on bootstrap `MapIndex = 42`
3. the server-side runtime relocates player B from `MapIndex = 1` to `MapIndex = 42`
4. player A receives `CHARACTER_DEL` for player B
5. player B receives:
   - `CHARACTER_DEL` for player A
   - a visibility burst for player C:
     - `CHARACTER_ADD`
     - `CHAR_ADDITIONAL_INFO`
     - `CHARACTER_UPDATE`
6. player C receives the same visibility burst for player B, using player B's relocated snapshot on the destination `MapIndex`

If the relocation keeps the player on the same `MapIndex`, the runtime updates the stored snapshot but does not emit any visibility rebuild frames.

## Why this slice exists

The project needs a first real primitive for world migration before freezing an end-to-end warp contract.

That primitive is:
- removing peers that are no longer visible after a map change
- bootstrapping peers that become visible on the destination map
- reusing already-owned visibility packets instead of inventing a second bootstrap family

This gives the runtime a concrete map-change behavior without pretending that the full legacy warp/loading choreography is already understood.

## Explicit non-goals

This slice does not yet add:

- a client packet for requesting a warp
- a server packet that freezes final warp/loading semantics
- persistence writeback for relocated snapshots across fresh sessions
- channel topology or inter-channel migration
- sector/range/AOI culling
- NPC, mob, item, or generic entity relocation
- reconnect semantics across map transfer
