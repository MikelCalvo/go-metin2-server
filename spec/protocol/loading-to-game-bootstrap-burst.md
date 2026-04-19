# Loading-to-game bootstrap burst

This document freezes the current server-owned burst emitted when the client leaves `LOADING` and enters `GAME`.

The goal of this slice is narrow:
- accept `ENTERGAME` in `LOADING`
- transition the session to `GAME`
- emit one deterministic selected-character bootstrap burst first
- append already-visible peer bootstrap frames only after that self burst

This slice is intentionally small.
It does not yet freeze the full long-term world-entry contract for items, quickslots, NPCs, mobs, scripted warps, or loading-screen choreography.

## Covered packets

- `ENTERGAME` client -> server (`0x0204`)
- `PHASE(GAME)` server -> client (`0x0008` carrying `GAME`)
- `CHARACTER_ADD` server -> client (`0x0205`)
- `CHAR_ADDITIONAL_INFO` server -> client (`0x0207`)
- `CHARACTER_UPDATE` server -> client (`0x0209`)
- `PLAYER_POINT_CHANGE` server -> client (`0x0215`)

## Working flow

The current project-owned behavior is:

1. the session is already in `LOADING`
2. the client sends `ENTERGAME`
3. the server validates the request and transitions to `GAME`
4. the server emits one ordered burst:
   - `PHASE(GAME)`
   - selected-character bootstrap frames
   - trailing peer-visibility frames for already-visible peers, if any

## Selected-character bootstrap frames

The current selected-character bootstrap burst is emitted in this exact order:

1. `CHARACTER_ADD`
2. `CHAR_ADDITIONAL_INFO`
3. `CHARACTER_UPDATE`
4. `PLAYER_POINT_CHANGE`

These four frames belong to the selected character only.
They are emitted before any trailing peer frames.

## Trailing peer frames

If the entering player already has visible peers in the current bootstrap runtime, those peer bootstrap frames are appended only after the selected-character burst.

The current trailing peer visibility burst reuses the existing visibility packets and may currently include:
- `CHARACTER_ADD`
- `CHAR_ADDITIONAL_INFO`
- `CHARACTER_UPDATE`

This ordering is intentional:
- the client sees its own selected-character bootstrap first
- peer visibility is layered on top afterwards

## Internal contract

The current server-owned world-entry flow models the burst in two buckets:
- `bootstrap_frames`
- `trailing_frames`

The emitted wire order is:
1. `PHASE(GAME)`
2. `bootstrap_frames...`
3. `trailing_frames...`

That split exists to keep the self bootstrap deterministic while still allowing peer visibility to grow incrementally in later slices.

## Slice scope

This slice freezes:
- acceptance of `ENTERGAME` from `LOADING`
- the `LOADING -> GAME` phase transition
- the exact selected-character bootstrap order after `PHASE(GAME)`
- the rule that peer visibility frames, when present, are appended after the selected-character burst

It does not yet freeze:
- item or quickslot bootstrap
- NPC or mob insertion
- final loading-screen / warp choreography
- cross-channel migration behavior
- any claim that the current secure handshake is compatibility-complete for a real client
