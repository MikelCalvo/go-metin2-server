# Post-floor compatible partial-merge `SAFEBOX_ITEM_MOVE` restart recovery — 2026-09-01

## Objective

Close the remaining compatible partial-merge safebox-cell move recovery twin after
whole-stack / empty-destination partial-split `SAFEBOX_ITEM_MOVE` restart recovery
already landed: prove `/restart_here` / `/restart_town` restore a usable live
owner that can freshly `/open_safebox` and complete an ordinary partial-count
compatible same-`vnum` `SAFEBOX_ITEM_MOVE` merge.

## Contract frozen by this slice

1. Seed two compatible carried stacks (`count = 4` and `count = 3`), open lab
   `/open_safebox`, accept `SAFEBOX_CHECKIN` into safebox cells `0` and `1`, then
   drive the owner to the retaliation HP floor so the floor edge closes the open
   presentation with `CloseSafebox`.
2. Later packet `SAFEBOX_ITEM_MOVE` (`0 -> 1`, count `2`) fails closed with no
   safebox frames and no inventory / gold / persistence mutation while dead.
3. After `/restart_here` restores live HP, `/open_safebox` rematerializes both
   remembered cells (`count = 4` and `count = 3`) and the same partial
   `SAFEBOX_ITEM_MOVE` emits ordinary dual `SAFEBOX_SET` (source remainder `2` +
   merged destination `5`), leaving carried inventory empty beside recovered
   MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same reopen + partial-merge path likewise succeeds and persists beside
   recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSafeboxItemMovePartialMergeFailsClosed`
   - `TestGameSessionFlowPostFloorSafeboxItemMovePartialMergeFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- widening into mall / whole-stack compatible-merge restart twins in this same commit
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxItemMovePartialMergeFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_safebox_item_move_partial_merge_restart_recovery_test.go
git diff --check
```
