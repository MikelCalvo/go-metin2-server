# Kill After Chase Still-Dead Daemon Restart — 2026-09-08

## Objective

Close the remaining Track A still-dead persistence coverage gap where a live
killing hit after a within_radius chase displace already kept the corpse at the
death coords in-process, and daemon-restart still-dead restore already carried
`combat_current_hp=0` plus absolute `respawn_ready_at`, but focused restart
coverage only killed an at-home dummy.

## Status

Done for bootstrap scope as ordinary GREEN twin coverage:
`TestGameRuntimeKillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart`.

## Why now

- At-home daemon-restart still-dead restore is GREEN
  (`TestGameRuntimeContentSpawnGroupStillDeadPersistsAcrossDaemonRestart`).
- Same-process kill-after-chase still-dead / authored-home respawn is GREEN
  (`TestGameRuntimeKillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome`).
- Persist/restore already writes current X/Y with the still-dead overlay.
  Without a displaced restart twin, a regression could rematerialize the
  corpse at authored home or arm homeward/return during the dead interval.

## Focused coverage

- `TestGameRuntimeKillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart`

```bash
go test ./internal/minimal -run 'TestGameRuntimeKillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart$' -count=1
```

Neighbor stay-green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(KillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart|KillingHitAfterChaseDisplaceKeepsStillDeadAtDeathCoordsAndRespawnsAtAuthoredHome|ContentSpawnGroupStillDeadPersistsAcrossDaemonRestart)$' -count=1
```

## Contract owned by this slice

1. After a within_radius chase displace, the killing hit persists
   `combat_current_hp=0`, absolute `respawn_ready_at`, current death coords,
   and authored `spawn_home`.
2. A clean `gamed` restart rematerializes that same authored `spawn_group_ref`
   as still-dead / non-targetable at the death coords (not authored home).
3. Restore does not arm chase, homeward, or return-step; engagement /
   selected-target stay fail-closed.
4. Late EnterGame during the remaining deadline bootstraps add/info/update plus
   trailing `GC DEAD` at the death coords, and a later homeward delay still
   emits no corpse `MOVE`.
5. Due respawn rebuilds at authored home and clears the still-dead persistence
   fields.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- still-dead `return_required` restore must not arm an internal return-step
  deadline from the stale live register snapshot (next Track A seam)
- cross-map MOVE / `GC WARP` for mobs, pack AI, pathfinding, target switching
