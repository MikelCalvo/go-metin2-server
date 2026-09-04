# Combat-Range Loss Homeward After Chase — 2026-09-04

## Objective

Close the Track A engagement-release coverage gap where hit-armed combat-range
loss already cleared selected-target ownership through
`clearInvalidActiveCombatTargetAfterMovement` → `clearActiveCombatTarget` (and
therefore cleared chase and armed within_radius homeward after a chase
displace), but only the training-dummy clear-target twin and the hit-armed
"survives walk outside aggro" twin owned related behavior.

## Why now

- Spec + asymmetry freeze already name **combat-range loss** as an explicit
  hit-armed release boundary beside `TARGET(0)`, death floor, disconnect /
  transfer, return recovery, and operator update.
- Distances make this distinct from owned aggro walk-away: aggro `200`,
  combat-target `300`, leash `400`.
- Owned twin proves hit-armed chase **survives** walk outside aggro while still
  inside combat range
  (`TestGameRuntimeHitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius`).
- Only existing range-loss proof is training-dummy
  (`TestGameSessionFlowStaticActorCombatTargetClearsWhenSelectedDummyLeavesCombatRange`)
  — no chase, no homeward.
- Live path already exists; without a focused twin, a future regression on the
  movement release helper could silently leave chase-displaced mobs sitting
  forever off-home after combat-range loss.

## Focused coverage

- `TestGameRuntimeCombatRangeLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`

```bash
go test ./internal/minimal -run 'TestGameRuntimeCombatRangeLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

Neighbor stay-green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(HitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius|CombatRangeLossClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace|ProximityWalkAwayClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace)$' -count=1
```

## Contract owned by this slice

1. After a within_radius chase displace under hit-armed / still-selected
   engagement, walking outside combat-target range (`300`) while remaining
   inside visibility clears the selected combat target with one self-only
   `TARGET(0, 0)`.
2. The same clear path releases `engaged_by`, clears pending chase, and arms
   one pending within_radius homeward deadline.
3. Due homeward still fans retained-viewer `MOVE` back to authored home and
   clears the deadline at `at_home`.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
- inventing a new leave scheduler; this is coverage for already-live combat-
  range loss clear
- AOI / visibility-loss chase/homeward twin (next evidence-backed seam)
