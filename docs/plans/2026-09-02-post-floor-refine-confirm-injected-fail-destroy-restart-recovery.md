# Post-floor injected-fail destroy refine confirm restart recovery — 2026-09-02

## Objective

Close the last deferred sibling of the post-floor refine confirm restart
matrix: prove `/restart_here` / `/restart_town` restore a usable live owner
that can freshly preview a refineable carried item with authored
`probability` in `1..99` (no `keep_on_fail` / `fail_result_vnum`), then
complete a matching injected-roll failure confirm that destroys the source
while consuming materials/gold.

## Contract to freeze / prove

1. Seed one refineable carried source (`probability = 75`, no keep/downgrade
   companions) plus one fully-consumed material stack, with item quickslots
   bound to both cells and one unrelated skill quickslot retained, drive the
   owner to the retaliation HP floor, then later preview `REFINE` fails closed
   with no `REFINE_INFORMATION_NEW` and no inventory / gold / quickslot /
   persistence mutation while dead.
2. A matching confirm while still dead likewise fails closed with no frames and
   no mutation.
3. After `/restart_here` restores live HP, the same preview emits one
   `REFINE_INFORMATION_NEW`, and an injected failing roll (`76`) confirm emits
   the ordinary 6-frame destroy burst (`ITEM_DEL` material + material
   `QUICKSLOT_DEL` + source `ITEM_DEL` + source `QUICKSLOT_DEL` + gold
   `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND` `RefineFailed <type>`),
   persisting emptied inventory, consumed materials, reduced gold, and cleared
   item quickslots beside recovered MaxHP while retaining the unrelated skill
   quickslot.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same preview + injected-fail confirm path likewise destroys and persists
   beside recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorRefineConfirmInjectedFailDestroySucceedsAfterRestartHere`
   - `TestGameSessionFlowPostFloorRefineConfirmInjectedFailDestroySucceedsAfterRestartTown`

## Explicit non-goals

- inventing a death-specific refine packet family
- widening into catalyst / keep-on-fail / fail-result / injected-success twins
  in this same commit (those siblings are already owned)
- mall / real `ITEM_GIVE` transfer / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorRefineConfirmInjectedFailDestroySucceedsAfterRestart' -count=1
gofmt -w internal/minimal/post_floor_refine_confirm_injected_fail_destroy_restart_recovery_test.go
git diff --check
```
