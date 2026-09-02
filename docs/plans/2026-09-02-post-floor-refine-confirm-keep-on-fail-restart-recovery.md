# Post-floor `keep_on_fail` refine confirm restart recovery — 2026-09-02

## Objective

Close the next deferred sibling of the already-owned post-floor refine confirm
restart twins: prove `/restart_here` / `/restart_town` restore a usable live
owner that can freshly preview a refineable carried item with authored
`probability` in `1..99` + `keep_on_fail`, then complete a matching injected-roll
failure confirm that keeps the source while consuming materials/gold.

## Contract frozen by this slice

1. Seed one refineable carried source (`probability = 75`, `keep_on_fail = true`)
   plus one fully-consumed material stack, with item quickslots bound to both
   cells and one unrelated skill quickslot retained, drive the owner to the
   retaliation HP floor, then later preview `REFINE` fails closed with no
   `REFINE_INFORMATION_NEW` and no inventory / gold / quickslot / persistence
   mutation while dead.
2. A matching confirm while still dead likewise fails closed with no frames and
   no mutation.
3. After `/restart_here` restores live HP, the same preview emits one
   `REFINE_INFORMATION_NEW`, and an injected failing roll (`76`) confirm emits
   the ordinary 4-frame keep-failure burst (`ITEM_DEL` material + material
   `QUICKSLOT_DEL` + gold `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND`
   `RefineFailed <type>`), persisting the kept source, consumed materials,
   reduced gold, and cleared material quickslot beside recovered MaxHP while
   retaining the source item quickslot and unrelated skill quickslot.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same preview + injected-fail confirm path likewise keeps the source and
   persists beside recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorRefineConfirmKeepOnFailSucceedsAfterRestartHere`
   - `TestGameSessionFlowPostFloorRefineConfirmKeepOnFailSucceedsAfterRestartTown`

## Explicit non-goals

- inventing a death-specific refine packet family
- widening into catalyst / `fail_result_vnum` / injected-success restart twins
  in this same commit
- mall / real `ITEM_GIVE` transfer / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorRefineConfirmKeepOnFailSucceedsAfterRestart' -count=1
gofmt -w internal/minimal/post_floor_refine_confirm_keep_on_fail_restart_recovery_test.go
git diff --check
```
