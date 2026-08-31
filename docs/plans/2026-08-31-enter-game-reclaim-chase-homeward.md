# EnterGame Reclaim Homeward After Chase — 2026-08-31

## Objective

Close the Track A engagement-release gap where EnterGame reclaim / Join dropped
stale practice-mob `engaged_by` (and pruned chase) after a within_radius chase
displace but never re-armed pending homeward, unlike slash leave / transfer /
`TARGET(0)` / death-floor.

## Why now

- Slash leave and transfer already snapshot-or-order combat clear so
  within_radius homeward re-arms after chase displace.
- EnterGame reclaim only called `pruneSpawnGroupChaseStepSchedules()` after Join
  `removeStaleOwnership`, so displaced mobs could sit forever off-home until a
  later unrelated release path.
- Spec already lists EnterGame reclaim among homeward arm sources; live Join was
  the missing consumer.

## Focused coverage

- `TestGameRuntimeEnterGameReclaimClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace`

```bash
go test ./internal/minimal -run 'TestGameRuntimeEnterGameReclaimClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace$' -count=1
```

Neighbor leave/transfer twins stay green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(SlashQuit|SlashLogout|PhaseSelect|Transfer)ClearsPendingSpawnGroupChaseAndArmsHomewardAfterChaseDisplace|TestGameRuntimeEnterGameReclaimClearsPendingSpawnGroupChase' -count=1
```

## Contract owned by this slice

1. Before `sharedWorld.Join`, snapshot spawn-group entity IDs engaged by
   reclaimable stale duplicate subjects for the entering character.
2. After a successful Join that reclaimed those subjects, clear pending chase,
   sync within_radius homeward, and persist combat state for each snapped actor.
3. Prune chase and homeward schedules before encoding EnterGame visibility.
4. Due homeward still fans retained-viewer `MOVE` back to authored home.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
