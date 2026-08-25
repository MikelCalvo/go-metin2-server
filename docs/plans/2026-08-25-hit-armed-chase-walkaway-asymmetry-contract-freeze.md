# Hit-Armed Chase Walk-Away Asymmetry Contract Freeze — 2026-08-25

## Objective

Freeze the intentional Track A asymmetry between:

- proximity-only engagement leave-radius walk-away (releases `engaged_by`,
  clears chase / delayed retaliation, invents no `TARGET(0, 0)`)
- hit-armed / selected-target engagement walk outside aggro while still
  selected (keeps engagement + pending chase until an owned release boundary)

This is a docs/spec contract freeze only. No production code change. The live
runtime already gates proximity leave-radius release behind
`activeCombatTargetVID == 0` in `clearInvalidActiveCombatTargetAfterMovement`.

## Contract owned by this freeze

1. Proximity-only engagement (no selected combat target) that walks outside the
   actor's effective aggro radius while still inside leash/visibility:
   - releases `engaged_by`
   - clears pending chase and cancels delayed retaliation
   - invents no self `TARGET(0, 0)`
   - already proven by `TestGameRuntimeProximityAggroWalkAwayReleasesEngagementAndCancelsDelayedRetaliation`
2. Hit-armed engagement that still holds a selected combat target and walks
   outside aggro while remaining inside combat-target range / leash /
   visibility:
   - does **not** release engagement solely because aggro radius was left
   - does **not** clear a pending chase deadline solely because aggro radius was left
   - delayed retaliation / selected-target ownership continue under the already
     owned hit-engagement rules until an explicit release boundary
     (`TARGET(0)`, death floor, disconnect/transfer, return recovery, operator
     update, combat-range loss clear, etc.)
3. No new packets, aggro hysteresis, pack AI, absolute schedule rematerialize,
   or pathfinding.

## Why RED is deferred

A focused twin proof for (2) would currently pass against the already-live
`activeCombatTargetVID != 0` gate. Opening that as "RED" would be dishonest.
After this freeze lands green on `lane/world`, the next implementation slice can
add the twin proof as ordinary GREEN coverage (or stop at RED only if a real
missing-implementation gap appears while writing it).

## Focused follow-on coverage

Landed as ordinary GREEN twin (not dishonest RED):
`TestGameRuntimeHitArmedSpawnGroupChaseSurvivesOwnerWalkOutsideAggroRadius`.
See [hit-armed chase walkaway twin](2026-08-25-hit-armed-chase-walkaway-twin.md).

## What this is not yet

- ~~absolute chase / return / homeward deadline rematerialize across restart~~ Cancelled for Track A bootstrap as re-arm-from-now: see [absolute deadline rematerialize contract freeze](2026-08-25-absolute-chase-return-homeward-deadline-rematerialize-contract-freeze.md)
- ~~multi-step homeward cadence twin~~ Done: see [multi-step homeward cadence twin](2026-08-25-multi-step-homeward-cadence-twin.md)
- ~~chase replan twin that moves the owner between arm and first due step~~ Done: see [chase replan owner-moved twin](2026-08-25-chase-replan-owner-moved-between-arm-and-due.md)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
