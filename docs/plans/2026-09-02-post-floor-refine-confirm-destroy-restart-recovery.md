# Post-floor `probability = 0` refine confirm destroy restart recovery — 2026-09-02

## Objective

Close the deterministic destroy sibling of the already-owned
`probability = 100` post-floor refine confirm restart twins: prove
`/restart_here` / `/restart_town` restore a usable live owner that can freshly
preview a refineable carried item and complete a matching `probability = 0`
confirm with the ordinary destroy burst, including source/material quickslot
sync.

## Contract frozen by this slice

1. Seed one refineable carried source (`probability = 0`) plus one
   fully-consumed material stack, with item quickslots bound to both cells and
   one unrelated skill quickslot retained, drive the owner to the retaliation
   HP floor, then later preview `REFINE` fails closed with no
   `REFINE_INFORMATION_NEW` and no inventory / gold / quickslot / persistence
   mutation while dead.
2. A matching confirm while still dead likewise fails closed with no frames and
   no mutation.
3. After `/restart_here` restores live HP, the same preview emits one
   `REFINE_INFORMATION_NEW`, and the matching confirm emits the ordinary
   6-frame destroy burst (`ITEM_DEL` material + material `QUICKSLOT_DEL` +
   source `ITEM_DEL` + source `QUICKSLOT_DEL` + gold `PLAYER_POINT_CHANGE` +
   `CHAT_TYPE_COMMAND` `RefineFailed <type>`), persisting emptied inventory,
   consumed materials, reduced gold, and cleared item quickslots beside
   recovered MaxHP while retaining the unrelated skill quickslot.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same preview + confirm path likewise destroys and persists beside
   recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorRefineConfirmDestroySucceedsAfterRestartHere`
   - `TestGameSessionFlowPostFloorRefineConfirmDestroySucceedsAfterRestartTown`

## Explicit non-goals

- inventing a death-specific refine packet family
- widening into catalyst / `probability` `1..99` / keep-on-fail /
  fail-result restart twins in this same commit
- mall / real `ITEM_GIVE` transfer / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorRefineConfirmDestroySucceedsAfterRestart' -count=1
gofmt -w internal/minimal/post_floor_refine_confirm_destroy_restart_recovery_test.go
git diff --check
```
