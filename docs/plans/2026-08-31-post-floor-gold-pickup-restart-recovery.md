# Post-floor gold `ITEM_PICKUP` restart recovery — 2026-08-31

## Objective

Close the remaining gold-pickup recovery twin after packet `ITEM_PICKUP` already
owned quiet post-floor denial plus inventory-item pickup restart recovery, and
after gold `ITEM_DROP` already owned restart recovery: prove `/restart_here` and
`/restart_town` restore a usable live owner so the same gold `ITEM_PICKUP`
succeeds normally again.

## Contract frozen by this slice

1. Drop one owned gold marker while alive so a pending gold ground handle
   exists, then drive the owner to the retaliation HP floor with a
   content-loaded practice mob.
2. Later packet `ITEM_PICKUP` for that gold marker fails closed with:
   - no `GROUND_DEL` / gold `PLAYER_POINT_CHANGE` / `ITEM_GET` success burst
   - no ground-handle removal
   - no inventory / gold / quickslot / persistence mutation
3. After `/restart_here` restores live HP, the same gold `ITEM_PICKUP` emits the
   ordinary ground-delete + gold point-change + get burst, restores carried
   gold, and clears the pending marker.
4. After `/restart_town` restores live HP at the owned empire town position,
   drop one town-local gold marker and prove the same gold `ITEM_PICKUP`
   succeeds at the town-return coordinates beside recovered MaxHP; the
   source-map pre-floor gold marker stays pending (cross-map reclaim is out of
   scope).
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorGoldPickupFailsClosed`
   - `TestGameSessionFlowPostFloorGoldPickupFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific gold-pickup packet family
- widening into partial `ITEM_DROP2`, `QUICKSLOT_DEL` / `QUICKSLOT_SWAP`,
  merchant buy/sell, safebox check-in/out/move, refine confirm, or
  `ITEM_USE_TO_ITEM` restart twins in this same commit
- inventing cross-map reclaim of a source-map pending gold marker after town
  return
- changing already-owned live gold drop/pickup behavior
- changing already-owned inventory-item `ITEM_PICKUP` post-floor denial /
  recovery

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorGoldPickupFailsClosed' -count=1
gofmt -w internal/minimal/player_death_item_guard_test.go
git diff --check
```
