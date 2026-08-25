# Same-Map Chase / Return / Homeward MOVE AOI Membership — 2026-08-25

## Objective

Close the Track A multi-viewer AOI membership twins for same-map server-owned
spawn steps that already share `RelocateStaticActorTargetDiff` with the landed
operator/runtime position MOVE proof:

- due chase-step
- due return-step
- due homeward-step

## Contract owned by this slice

1. With radius AOI enabled, a due same-map chase / return / homeward step that
   moves the actor far enough to change membership queues:
   - `CHARACTER_DEL` to old-position-only viewers
   - one retained-viewer `MOVE` to viewers that stay inside radius
   - `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` to newly-visible
     destination viewers
2. Chase preserves engagement / selected-target ownership across the membership
   flush (same rule as the retained-only chase MOVE fanout proofs).
3. Return / homeward continue to use the already-owned recovery semantics;
   this slice only proves membership symmetry on the shared relocate path.
4. Absolute pending chase / return / homeward deadline persistence across
   daemon restart stays deferred: restore re-arms eligible schedules from now.
5. Cross-map MOVE / `GC WARP`, pack AI, synchronized respawn, and pathfinding
   stay cancelled / out of scope.

## Focused coverage

- `TestGameRuntimeFlushServerFramesAppliesDueSpawnGroupChaseStepQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd`
- `TestGameRuntimeFlushServerFramesAppliesDueSpawnGroupReturnStepQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd`
- `TestGameRuntimeFlushServerFramesAppliesDueSpawnGroupHomewardStepQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd`

```bash
go test ./internal/minimal -run 'TestGameRuntimeFlushServerFramesAppliesDueSpawnGroup(Chase|Return|Homeward)StepQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd$' -count=1
```

## What this is not yet

- absolute schedule rematerialize for chase / return / homeward across restart
- cross-map MOVE / `GC WARP`
- pack AI / synchronized respawn / pathfinding
- a new Track A evidence-backed seam beyond these membership twins
