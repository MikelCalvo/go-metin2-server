# Post-floor `fail_result_vnum` refine confirm restart recovery — 2026-09-02

## Objective

Close the next deferred sibling of the already-owned post-floor refine confirm
restart twins: prove `/restart_here` / `/restart_town` restore a usable live
owner that can freshly preview a refineable carried item with authored
`probability` in `1..99` + non-zero `fail_result_vnum`, then complete a matching
injected-roll failure confirm that downgrades the source while consuming
materials/gold.

## Contract frozen by this slice

1. Seed one refineable carried source (`probability = 75`,
   `fail_result_vnum` non-zero, `keep_on_fail` omitted/false) plus one
   fully-consumed material stack, with item quickslots bound to both cells and
   one unrelated skill quickslot retained, drive the owner to the retaliation
   HP floor, then later preview `REFINE` fails closed with no
   `REFINE_INFORMATION_NEW` and no inventory / gold / quickslot / persistence
   mutation while dead.
2. A matching confirm while still dead likewise fails closed with no frames and
   no mutation.
3. After `/restart_here` restores live HP, the same preview emits one
   `REFINE_INFORMATION_NEW`, and an injected failing roll (`76`) confirm emits
   the ordinary 5-frame downgrade-failure burst (`ITEM_DEL` material + material
   `QUICKSLOT_DEL` + source-cell `ITEM_SET(fail_result_vnum)` + gold
   `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND` `RefineFailed <type>`),
   persisting the downgraded source (same instance id/cell), consumed
   materials, reduced gold, and cleared material quickslot beside recovered
   MaxHP while retaining the source item quickslot and unrelated skill
   quickslot.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same preview + injected-fail confirm path likewise downgrades the source
   and persists beside recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorRefineConfirmFailResultVnumSucceedsAfterRestartHere`
   - `TestGameSessionFlowPostFloorRefineConfirmFailResultVnumSucceedsAfterRestartTown`

## Explicit non-goals

- inventing a death-specific refine packet family
- widening into catalyst / injected-success restart twins in this same commit
- mall / real `ITEM_GIVE` transfer / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorRefineConfirmFailResultVnumSucceedsAfterRestart' -count=1
gofmt -w internal/minimal/post_floor_refine_confirm_fail_result_vnum_restart_recovery_test.go
git diff --check
```
