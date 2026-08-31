# Post-floor `ITEM_PICKUP` restart recovery — 2026-08-31

## Objective

Close the remaining pickup recovery twin after packet `ITEM_DROP` /
`ITEM_DROP2` already owned quiet post-floor denial plus whole-stack /
gold drop restart recovery: prove `/restart_here` and `/restart_town`
restore a usable live owner so the same `ITEM_PICKUP` succeeds normally
again.

## Contract frozen by this slice

1. Drop one whole carried stack while alive so a pending owned ground handle
   exists, then drive the owner to the retaliation HP floor with a
   content-loaded practice mob.
2. Later packet `ITEM_PICKUP` fails closed with:
   - no `GROUND_DEL` / `ITEM_SET` / `ITEM_GET` success burst
   - no ground-handle removal
   - no inventory / gold / quickslot / persistence mutation
3. After `/restart_here` restores live HP, the same `ITEM_PICKUP` emits the
   ordinary ground-delete + item-set + get burst, restores the carried slot,
   and clears the pending handle.
4. After `/restart_town` restores live HP at the owned empire town position,
   drop one remaining carried stack at town and prove the same `ITEM_PICKUP`
   succeeds at the town-return coordinates beside recovered MaxHP; the
   source-map pre-floor ground handle stays pending (cross-map reclaim is
   out of scope).
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorItemPickupFailsClosed`
   - `TestGameSessionFlowPostFloorItemPickupFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific pickup packet family
- widening into gold pickup, partial `ITEM_DROP2`, quickslot, or sell restart
  twins in this same commit
- inventing cross-map reclaim of a source-map pending handle after town return
- changing already-owned live `ITEM_PICKUP` behavior

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorItemPickupFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
