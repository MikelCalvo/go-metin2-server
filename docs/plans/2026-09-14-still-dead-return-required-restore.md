# Still-Dead Return-Required Restore — 2026-09-14

## Objective

Close the remaining Track A still-dead restore gap where live `return_required`
daemon rematerialize already re-arms return-step from now, operator updates
already refuse to arm return-step while dead, and inspection/planner already
fail closed on `HP=0`, but `loadPersistedStaticActors` synced return-step from
the pre-still-dead register snapshot.

## Status

Done for bootstrap scope:
`TestGameRuntimeRestoreStillDeadReturnRequiredSpawnGroupDoesNotArmReturnStep`.

## Why now

- Live `return_required` restore re-arms from now
  (`TestGameRuntimeRestoreReturnRequiredSpawnGroupSchedulesReturnStep`).
- Same-profile operator/runtime updates of a still-dead spawn-backed actor
  already suppress return-step while respawn is authoritative.
- Chase-displaced still-dead daemon restart already keeps the corpse at death
  coords without arming homeward/return/chase
  (`TestGameRuntimeKillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart`).
- Without this restore twin, a still-dead `return_required` snapshot could arm
  an internal return-step deadline even though the planner cannot MOVE a
  zero-HP corpse.

## Focused coverage

- `TestGameRuntimeRestoreStillDeadReturnRequiredSpawnGroupDoesNotArmReturnStep`

```bash
go test ./internal/minimal -run 'TestGameRuntimeRestoreStillDeadReturnRequiredSpawnGroupDoesNotArmReturnStep$' -count=1
```

Neighbor stay-green:

```bash
go test ./internal/minimal -run 'TestGameRuntime(RestoreStillDeadReturnRequiredSpawnGroupDoesNotArmReturnStep|RestoreReturnRequiredSpawnGroupSchedulesReturnStep|KillingHitAfterChaseDisplaceStillDeadPersistsAcrossDaemonRestart)$' -count=1
```

## Contract owned by this slice

1. A clean `gamed` restart that rematerializes a still-dead spawn-backed actor
   already classified `return_required` (`combat_current_hp=0` + absolute
   `respawn_ready_at` + displaced current X/Y) must not arm an automatic
   return-step deadline.
2. Pending return-step inspection omits that corpse; the planner stays
   dead-gated on `HP=0`.
3. A due return-step flush must not MOVE the corpse toward authored home; the
   body stays at the persisted death coords through the remaining respawn
   deadline.
4. Live `return_required` restore continues to re-arm from now. Absolute
   chase/return/homeward due-at rematerialize stays cancelled.

## What this is not yet

- absolute chase / return / homeward due-at rematerialize across daemon restart
  (cancelled for Track A bootstrap as re-arm-from-now)
- cross-map MOVE / `GC WARP` for mobs, pack AI, pathfinding, target switching
