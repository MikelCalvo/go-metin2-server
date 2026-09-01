# Post-floor whole-stack compatible-merge `SAFEBOX_ITEM_MOVE` restart recovery — 2026-09-01

## Objective

Close the remaining whole-stack compatible-merge safebox-cell move recovery twin
after empty-destination relocate / partial-split / partial-merge `SAFEBOX_ITEM_MOVE`
restart recovery already landed: prove `/restart_here` / `/restart_town` restore a
usable live owner that can freshly `/open_safebox` and complete an ordinary
whole-stack compatible same-`vnum` `SAFEBOX_ITEM_MOVE` merge (`count = 0`).

## Contract frozen by this slice

1. Seed two compatible carried stacks (`count = 4` and `count = 3`), open lab
   `/open_safebox`, accept `SAFEBOX_CHECKIN` into safebox cells `0` and `1`, then
   drive the owner to the retaliation HP floor so the floor edge closes the open
   presentation with `CloseSafebox`.
2. Later packet `SAFEBOX_ITEM_MOVE` (`0 -> 1`, count `0`) fails closed with no
   safebox frames and no inventory / gold / persistence mutation while dead.
3. After `/restart_here` restores live HP, `/open_safebox` rematerializes both
   remembered cells (`count = 4` and `count = 3`) and the same whole-stack
   `SAFEBOX_ITEM_MOVE` emits ordinary `SAFEBOX_DEL` (source) + `SAFEBOX_SET`
   (merged destination `7`), leaving carried inventory empty beside recovered
   MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same reopen + whole-stack merge path likewise succeeds and persists beside
   recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSafeboxItemMoveWholeMergeFailsClosed`
   - `TestGameSessionFlowPostFloorSafeboxItemMoveWholeMergeFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox packet family
- widening into mall / refine catalysts / GD-DB `MYSHOP_PRICELIST`
- tip-`0023` myshop unit-prices scoped-replace GREEN (Track E follow-on)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxItemMoveWholeMergeFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_safebox_item_move_whole_merge_restart_recovery_test.go
git diff --check
```
