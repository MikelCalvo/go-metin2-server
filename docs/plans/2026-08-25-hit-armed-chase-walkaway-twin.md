# Hit-Armed Chase Walk-Away Twin — 2026-08-25

## Objective

Land the already-frozen Track A asymmetry twin as ordinary GREEN coverage:

- proximity-only leave-radius walk-away still releases engagement / chase
- hit-armed / still-selected owners that walk outside aggro while remaining
  inside combat-target range / leash / visibility keep engagement, selected
  target, and pending chase; delayed retaliation continues; the due chase
  retained-viewer `MOVE` still fires

## Contract owned by this slice

1. Reuse the already-frozen rules in `spawn-leash-bootstrap.md` (hit-armed vs
   proximity leave-radius asymmetry) and the prior docs freeze
   `2026-08-25-hit-armed-chase-walkaway-asymmetry-contract-freeze.md`.
2. No production code change is required: the live
   `activeCombatTargetVID != 0` gate in
   `clearInvalidActiveCombatTargetAfterMovement` already owns the behavior.
3. No new packets, aggro hysteresis, pack AI, absolute schedule rematerialize,
   or pathfinding.

## Focused coverage

- `TestGameRuntimeHitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius`

```bash
go test ./internal/minimal -run 'TestGameRuntimeHitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius$' -count=1
```

## What this is not yet

- absolute chase / return / homeward deadline rematerialize across daemon restart
- multi-step homeward cadence when displace > `max_step`
- chase replan twin that moves the owner between arm and first due step
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
