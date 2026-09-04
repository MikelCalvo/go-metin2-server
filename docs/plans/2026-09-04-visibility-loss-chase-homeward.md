# Visibility-Loss Homeward After Chase — 2026-09-04

## Objective

Close the remaining Track A engagement-release coverage gap where AOI /
visibility loss already clears selected-target ownership through the same
movement helper as combat-range loss (`clearInvalidActiveCombatTargetAfterMovement`
→ `clearActiveCombatTarget`), but only the training-dummy visibility clear twin
owned related behavior — no spawn-backed chase / homeward proof after a
within_radius chase displace.

## Status

Done for bootstrap scope as ordinary GREEN twin coverage:
`TestGameRuntimeVisibilityLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`.

## Why now

- Combat-range-loss chase/homeward is GREEN
  (`TestGameRuntimeCombatRangeLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`).
- Training-dummy visibility loss already proves `CHARACTER_DEL` + `TARGET(0, 0)`
  (`TestGameSessionFlowStaticActorCombatTargetClearsWhenSelectedDummyLeavesVisibility`).
- Spec / roadmap item 41 named this as the next evidence-backed seam after
  combat-range loss.
- Live path already clears through the same helper, so this was **ordinary GREEN
  twin coverage**, not a missing-implementation RED.

## Focused coverage

- `TestGameRuntimeVisibilityLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`

```bash
go test ./internal/minimal -run 'TestGameRuntimeVisibilityLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

Neighbor stay-green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(CombatRangeLoss|VisibilityLoss|ProximityWalkAway)ClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

## Contract owned by this slice

1. After a within_radius chase displace under hit-armed / still-selected
   engagement, walking outside visibility of the displaced actor queues
   `CHARACTER_DEL` plus one self-only `TARGET(0, 0)`.
2. The same clear path releases `engaged_by`, clears pending chase, and arms
   one pending within_radius homeward deadline.
3. Due homeward still fans retained-viewer `MOVE` back to authored home and
   clears the deadline at `at_home`.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
- inventing a new leave scheduler; this is coverage for already-live AOI clear
