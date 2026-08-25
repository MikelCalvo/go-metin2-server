# Daemon-Restart Chase-Unarmed Re-arm-From-Now Twin — 2026-08-25

## Objective

Land the optional ordinary GREEN composite twin after the absolute
chase/return/homeward deadline rematerialize freeze: one restart proves
eligible return/homeward re-arm-from-now beside chase staying unarmed and
engagement / selected-target ownership staying fail-closed.

## Why this is GREEN, not RED

`loadPersistedStaticActors` already re-arms eligible return/homeward through
`syncSpawnGroupReturnStepSchedule` /
`syncSpawnGroupHomewardStepScheduleForEntity`, and chase cannot rematerialize
without engagement ownership (fail-closed across restart). Absolute mid-timer
due-at rematerialize RED remains cancelled for Track A bootstrap. See
[absolute deadline rematerialize contract freeze](2026-08-25-absolute-chase-return-homeward-deadline-rematerialize-contract-freeze.md).

## Focused coverage

- `TestGameRuntimeDaemonRestartRearmsReturnAndHomewardFromNowAndLeavesChaseUnarmed`

```bash
go test ./internal/minimal -run 'TestGameRuntimeDaemonRestartRearmsReturnAndHomewardFromNowAndLeavesChaseUnarmed$' -count=1
```

## Contract owned by this slice

1. One chase-displaced live actor rematerializes `within_radius` and restore
   arms pending homeward from **now**.
2. One `return_required` displaced actor rematerializes still outside leash and
   restore arms pending return-step from **now**.
3. Chase stays unarmed across restart for both actors even when chase was armed
   pre-restart on the within_radius actor.
4. Engagement / selected-target ownership stay fail-closed (empty combat-target
   snapshots after reload).
5. No new packets, scheduler, absolute due-at fields, cross-map MOVE/WARP, or
   pack AI.

## What this is not yet

- inventing absolute mid-timer chase / return / homeward due-at rematerialize
  (cancelled for Track A bootstrap)
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
- any speculative RED without a new evidence-backed seam beyond the freeze
