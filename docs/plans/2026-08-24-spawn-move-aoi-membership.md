# Same-Map Spawn MOVE AOI Membership — 2026-08-24

## Objective

Close the already-frozen Track A remove/add visibility membership contract for
same-map live spawn-backed operator/runtime position MOVE: old-position-only
viewers get `CHARACTER_DEL`, newly-visible viewers get the ordinary
add/info/update burst, and retained viewers still get `MOVE` only.

## Contract owned by this slice

1. With radius AOI enabled, a live same-map spawn-backed position-only
   `UpdateStaticActor` that moves the actor far enough to change membership
   queues:
   - `CHARACTER_DEL` to old-position-only viewers
   - one retained-viewer `MOVE` to viewers that stay inside radius
   - `CHARACTER_ADD` + `CHAR_ADDITIONAL_INFO` + `CHARACTER_UPDATE` to newly-visible
     destination viewers
2. Runtime continues to reuse `RelocateStaticActorTargetDiff` already shared by
   chase / return-step / return-home / homeward; this slice does not invent a
   second membership helper.
3. Presentation/name/race refreshes, dead trailing-`DEAD` refreshes, respawn
   rebuild, content-bundle replacement, and cross-map return stay on
   delete/readd.
4. Absolute pending chase / return / homeward deadline persistence across
   daemon restart stays deferred: restore re-arms eligible schedules from now.

## Focused coverage

- `TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPositionQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd`

```bash
go test ./internal/minimal -run 'TestGameRuntimeUpdateStaticActorSameMapSpawnGroupPositionQueuesOldPositionOnlyDeleteAndNewlyVisibleAdd$' -count=1
```

## What this is not yet

- ~~dedicated multi-viewer chase / return / homeward AOI twins (optional symmetry;
  runtime path already shared)~~ Done: see [spawn step AOI membership](2026-08-25-spawn-step-aoi-membership.md).
- absolute schedule rematerialize for chase / return / homeward across restart
- cross-map MOVE / `GC WARP`
- pack AI / synchronized respawn / pathfinding
