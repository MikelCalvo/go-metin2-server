# Abrupt Close Homeward After Chase — 2026-09-01

## Objective

Close the Track A engagement-release coverage gap where abrupt socket close /
`onClose` already cleared practice-mob engagement with clear-then-Leave (and
therefore armed within_radius homeward) after a chase displace, but only slash
`/quit` / `/logout` / `/phase_select` had focused twins claiming that behavior.

## Why now

- Slash leave, transfer, EnterGame reclaim, death-floor, and `TARGET(0)` already
  own chase clear + within_radius homeward after chase displace.
- Spec and slash-leave comments already treat abrupt close as the reference
  clear-then-Leave order; live `onClose` matched that order.
- Without a focused twin, a future regression on the close hook could silently
  leave chase-displaced mobs sitting forever off-home after disconnect.

## Focused coverage

- `TestGameRuntimeAbruptCloseClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`

```bash
go test ./internal/minimal -run 'TestGameRuntimeAbruptCloseClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

Neighbor leave twins stay green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(SlashQuit|SlashLogout|PhaseSelect|AbruptClose)ClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

## Contract owned by this slice

1. After a within_radius chase displace, abrupt `closeSessionFlow` / session
   `onClose` clears pending chase for the engaged spawn-backed actor.
2. The same close path arms one pending within_radius homeward deadline.
3. Due homeward still fans retained-viewer `MOVE` back to authored home and
   clears the deadline at `at_home`.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
- inventing a new leave scheduler; this is coverage for already-live onClose
