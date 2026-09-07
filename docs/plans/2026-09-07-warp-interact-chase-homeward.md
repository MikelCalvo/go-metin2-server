# Warp INTERACT Homeward After Chase — 2026-09-07

## Objective

Close the remaining Track A engagement-release coverage gap where authored warp
`INTERACT` already reused `applySelectedCharacterTransfer` (and therefore
cleared chase and armed within_radius homeward after a chase displace), but
only operator `RelocateCharacter` / transfer owned the focused chase/homeward
twin. The existing warp proof only asserted `TARGET(0, 0)` plus aggro release
on an at-home dummy.

## Status

Done for bootstrap scope as ordinary GREEN twin coverage:
`TestGameRuntimeWarpInteractClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`.

## Why now

- Transfer / relocate chase/homeward is GREEN
  (`TestGameRuntimeTransferClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`).
- Authored warp INTERACT is the playable-vertical client ingress onto that same
  helper (`rebootstrap = true`), not the operator relocator (`rebootstrap = false`).
- Spec already names owner transfer/warp among chase-clear / homeward-arm
  sources; live `HandleInteraction` already called `applySelectedCharacterTransfer`.
- Without a focused twin, a future regression on the INTERACT/warp branch could
  silently leave chase-displaced mobs sitting forever off-home after NPC warp.

## Focused coverage

- `TestGameRuntimeWarpInteractClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`

```bash
go test ./internal/minimal -run 'TestGameRuntimeWarpInteractClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

Neighbor stay-green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(Transfer|WarpInteract)ClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

## Contract owned by this slice

1. After a within_radius chase displace under hit-armed engagement, authored
   warp `INTERACT` onto another map returns one self-only `TARGET(0, 0)` in the
   rebootstrap response and moves the owner to the warp destination.
2. The same path releases `engaged_by`, clears pending chase, and arms one
   pending within_radius homeward deadline.
3. Due homeward still fans retained-viewer `MOVE` back to authored home and
   clears the deadline at `at_home`.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map MOVE / `GC WARP` for mobs, pack AI, pathfinding, target switching
- inventing a new leave scheduler; this is coverage for already-live warp INTERACT
- MOVE / SYNC_POSITION bootstrap transfer-trigger chase/homeward twin (same
  helper; ordinary GREEN follow-on, not dishonest RED)
