# Multi-Step Homeward Cadence Twin — 2026-08-25

## Objective

Land the already-live within-radius homeward executor re-arm across more than
one `max_step=100` beat when chase leaves the actor farther than one step from
authored home as ordinary GREEN coverage.

## Why this is GREEN, not RED

`stepSpawnGroupHomeward` / `flushDueSpawnGroupHomewardSteps` already re-schedule
when the stepped actor remains `within_radius` and the step is incomplete. Spec
already requires “re-arm while still eligible `within_radius`”. Opening this as
RED would be dishonest against already-live gating.

Absolute chase/return/homeward deadline rematerialize across daemon restart
stays deferred (re-arm-from-now) and would need a docs freeze before any RED.

## Focused coverage

- `TestGameRuntimeFlushServerFramesAppliesMultiStepSpawnGroupHomewardCadenceAfterChaseDisplaceBeyondMaxStep`

```bash
go test ./internal/minimal -run 'TestGameRuntimeFlushServerFramesAppliesMultiStepSpawnGroupHomewardCadenceAfterChaseDisplaceBeyondMaxStep$' -count=1
```

## Contract owned by this slice

1. Two chase beats displace the engaged practice mob to +200 from authored home
   while remaining `within_radius`.
2. Engagement release (`TARGET(0)` after leaving aggro) arms the owned `1s`
   homeward deadline.
3. First due homeward applies retained-viewer `MOVE` +100 toward home and
   re-arms.
4. Second due homeward lands on authored home, classifies `at_home`, and clears
   the pending homeward deadline.
5. No new packets, scheduler, absolute deadline rematerialize, cross-map
   MOVE/WARP, or pack AI.

## What this is not yet

- ~~chase replan twin that moves the owner between arm and first due step~~ Done: see [chase replan owner-moved twin](2026-08-25-chase-replan-owner-moved-between-arm-and-due.md)
- ~~absolute chase / return / homeward deadline rematerialize across daemon restart~~ Cancelled for Track A bootstrap as re-arm-from-now: see [absolute deadline rematerialize contract freeze](2026-08-25-absolute-chase-return-homeward-deadline-rematerialize-contract-freeze.md)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
