# Chase Replan Owner-Moved Twin — 2026-08-25

## Objective

Land the already-live chase executor replan path as ordinary GREEN coverage:
plans from the engaged owner's live coordinates at flush time when the owner
moves between chase arm and the first due beat.

## Why this is GREEN, not RED

`stepSpawnGroupChase` already resolves `ownerPos` from the live engaged owner
character at flush (`playerCharacter` → `PlanSpawnGroupChaseStep`). Spec already
says a due chase step “resolves the current engaged owner's live position”.
Opening this as RED would be dishonest against already-live gating.

Absolute chase/return/homeward deadline rematerialize across daemon restart
stays deferred (re-arm-from-now) and would need a docs freeze before any RED.

## Focused coverage

- `TestGameRuntimeFlushServerFramesReplansSpawnGroupChaseTowardOwnerMovedBetweenArmAndDue`

```bash
go test ./internal/minimal -run 'TestGameRuntimeFlushServerFramesReplansSpawnGroupChaseTowardOwnerMovedBetweenArmAndDue$' -count=1
```

## Contract owned by this slice

1. Accepted hit arms the owned `5s` chase deadline while the owner is still near
   authored home (for example +100 on X).
2. Before the deadline fires, the owner walks farther along the same axis (for
   example to +200) while remaining inside combat-target range / leash /
   visibility so engagement and the pending chase row stay armed.
3. The due chase flush plans toward the live post-move owner coords and emits
   one retained-viewer `MOVE` onto that updated position (not the arm-time
   snapshot).
4. Engagement / selected-target ownership stay preserved.
5. No new packets, scheduler, absolute deadline rematerialize, cross-map
   MOVE/WARP, or pack AI.

## What this is not yet

- absolute chase / return / homeward deadline rematerialize across daemon restart
- cross-map MOVE / `GC WARP`, pack AI, pathfinding, target switching
