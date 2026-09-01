# Proximity Walk-Away Homeward After Chase — 2026-09-01

## Objective

Close the Track A engagement-release coverage gap where proximity leave-radius
walk-away already cleared chase and synced within_radius homeward after a chase
displace, but the existing proximity walk-away twin only asserted engagement /
chase clear / retaliation cancel from an at-home posture.

## Why now

- Spec already lists proximity leave-radius walk-away among homeward arm sources
  beside `TARGET(0)`, leave/logout/close, transfer, EnterGame reclaim, and owner
  death floor.
- Live `clearInvalidActiveCombatTargetAfterMovement` already calls
  `syncSpawnGroupHomewardStepScheduleForEntity` for released proximity actors.
- Without a focused twin on the chase-displaced path, a future regression on that
  movement release helper could silently leave chase-displaced mobs sitting
  forever off-home after walk-away.

## Focused coverage

- `TestGameRuntimeProximityWalkAwayClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`

```bash
go test ./internal/minimal -run 'TestGameRuntimeProximityWalkAwayClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

Neighbor proximity / leave twins stay green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(ProximityWalkAwayClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace|ProximityAggroWalkAwayReleasesEngagementAndCancelsDelayedRetaliation|AbruptCloseClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace|HitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius)$' -count=1
```

## Contract owned by this slice

1. After a within_radius chase displace under proximity-only engagement (no
   selected combat target), walking outside effective aggro releases `engaged_by`
   without inventing self `TARGET(0, 0)`.
2. The same walk-away path clears pending chase and arms one pending within_radius
   homeward deadline.
3. Due homeward still fans retained-viewer `MOVE` back to authored home and clears
   the deadline at `at_home`.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
- inventing a new leave scheduler; this is coverage for already-live proximity
  walk-away homeward sync
