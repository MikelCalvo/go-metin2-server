# Kill After Chase Respawn — 2026-09-08

## Objective

Close the remaining Track A still-dead / respawn coverage gap where a live
killing hit after a within_radius chase displace already cleared chase and
homeward on `HandleAttack`, restored authored home on due respawn, and encoded
still-dead visibility from current coordinates, but focused proofs were only
the at-home session flow and a synthetic timer-cleanup twin.

## Status

Done for bootstrap scope as ordinary GREEN twin coverage:
`TestGameRuntimeKillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome`.

## Why now

- At-home content spawn-group respawn is GREEN
  (`TestGameSessionFlowContentSpawnGroupPracticeMobRespawnsAfterServerDrivenDelayAndRequiresFreshReselect`).
- Synthetic pre-death chase/homeward deadline cleanup on respawn is GREEN
  (`TestGameRuntimeRespawnClearsStaleSpawnGroupChaseAndHomewardStepSchedules`).
- Live engagement-release after chase displace arms homeward
  (leave / transfer / warp / range-loss / visibility-loss). Actor death must
  **not** reuse that recovery: the corpse stays put until respawn.
- Spec already says dead actors do not arm chase/homeward and respawn restores
  authored home. Without a live kill-after-chase twin, a regression could
  homeward-MOVE the corpse or rebuild at the displaced death coords.

## Focused coverage

- `TestGameRuntimeKillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome`

```bash
go test ./internal/minimal -run 'TestGameRuntimeKillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome$' -count=1
```

Neighbor stay-green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(KillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome|RespawnClearsStaleSpawnGroupChaseAndHomewardStepSchedules)$|TestGameSessionFlowContentSpawnGroupPracticeMobRespawnsAfterServerDrivenDelayAndRequiresFreshReselect$' -count=1
```

## Contract owned by this slice

1. After a within_radius chase displace under hit-armed engagement, remaining
   killing hits emit `DEAD(target_vid)` + `TARGET(0, 0)` + damage-info.
2. The same path releases `engaged_by`, clears pending chase and homeward, and
   leaves the corpse at the death coords through the still-dead interval (no
   chase/homeward `MOVE` after the homeward delay).
3. Late EnterGame during that interval bootstraps add/info/update plus trailing
   `GC DEAD` at the death coords, not authored home, and stays non-targetable.
4. Due respawn rebuilds at authored home (`CHARACTER_DEL` + add/info/update),
   leaves chase/homeward unarmed, and requires fresh reselect.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- daemon-restart still-dead-at-displaced-coords is now GREEN
  (`TestGameRuntimeKillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart`;
  see `2026-09-08-kill-after-chase-still-dead-restart.md`)
- cross-map MOVE / `GC WARP` for mobs, pack AI, pathfinding, target switching
- inventing a new death scheduler; this is coverage for already-live kill +
  respawn after chase
