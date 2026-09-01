# Post-floor partial-count `SAFEBOX_ITEM_MOVE` restart recovery — 2026-09-01

## Objective

Close the remaining partial-split safebox-cell move recovery twin after
whole-stack `SAFEBOX_ITEM_MOVE` restart recovery and accepted check-in success
twins already landed: prove `/restart_here` / `/restart_town` restore a usable
live owner that can freshly `/open_safebox` and complete an ordinary
partial-count empty-destination `SAFEBOX_ITEM_MOVE`.

## Contract frozen by this slice

1. Seed one multi-count carried stack (`count = 5`), open lab `/open_safebox`,
   accept `SAFEBOX_CHECKIN` into safebox cell `0`, then drive the owner to the
   retaliation HP floor so the floor edge closes the open presentation with
   `CloseSafebox`.
2. Later packet `SAFEBOX_ITEM_MOVE` (`0 -> 1`, count `2`) fails closed with no
   safebox frames and no inventory / gold / persistence mutation while dead.
3. After `/restart_here` restores live HP, `/open_safebox` rematerializes the
   remembered cell (`count = 5`) and the same partial `SAFEBOX_ITEM_MOVE` emits
   ordinary dual `SAFEBOX_SET` (source remainder `3` + destination split `2`),
   leaving carried inventory empty beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same reopen + partial-split path likewise succeeds and persists beside
   recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSafeboxItemMovePartialSplitFailsClosed`
   - `TestGameSessionFlowPostFloorSafeboxItemMovePartialSplitFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- widening into mall / compatible partial-merge restart twins in this same commit
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxItemMovePartialSplitFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_safebox_item_move_partial_split_restart_recovery_test.go
git diff --check
```
