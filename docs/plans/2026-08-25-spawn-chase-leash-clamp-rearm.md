# Live Leash-Clamped Chase Clear + Hit Re-arm — 2026-08-25

## Objective

Close the already-frozen Track A chase executor gap for leash-clamped
complete steps:

- a due chase that stops on the effective leash boundary clears the pending
  chase deadline even when the engaged owner was not reached
- engagement / selected-target ownership stay preserved across that clamp
- a later same-engagement accepted hit re-arms the owned `5s` chase deadline
- after the actor is again safely inside leash (owner walked inward), the
  re-armed due chase applies a further retained-viewer `MOVE` step

## Contract owned by this slice

1. Reuse the already-frozen rules in `spawn-leash-bootstrap.md` (chase executor
   execution rules for leash-clamped complete steps + owned hit re-arm).
2. Use a profile-authored tight `leash_radius` so the live flush can reach the
   clamp boundary in two `max_step=100` beats without inventing pathfinding.
3. No new packets, scheduler, chase POST surface, absolute deadline
   rematerialize, cross-map MOVE/WARP, or pack AI.

## Focused coverage

- `TestGameRuntimeFlushServerFramesClearsLeashClampedSpawnGroupChaseStepAndRearmsOnHit`

```bash
go test ./internal/minimal -run 'TestGameRuntimeFlushServerFramesClearsLeashClampedSpawnGroupChaseStepAndRearmsOnHit$' -count=1
```

## What this is not yet

- absolute chase / return / homeward deadline persistence across daemon restart
- chase replan twin that moves the owner between arm and first due step
- multi-step homeward cadence when displace > `max_step`
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
