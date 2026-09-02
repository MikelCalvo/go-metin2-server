# Post-floor injected-success refine confirm restart recovery — 2026-09-02

## Objective

Close the deferred injected-success sibling of the already-owned post-floor
refine confirm restart twins: prove `/restart_here` / `/restart_town` restore a
usable live owner that can freshly preview a refineable carried item with
authored `probability` in `1..99`, then complete a matching injected-roll
success confirm that upgrades the source while consuming materials/gold.

## Contract frozen by this slice

1. Seed one refineable carried source (`probability = 75`, no `keep_on_fail` /
   `fail_result_vnum`) plus one fully-consumed material stack, with item
   quickslots bound to both cells and one unrelated skill quickslot retained,
   drive the owner to the retaliation HP floor, then later preview `REFINE`
   fails closed with no `REFINE_INFORMATION_NEW` and no inventory / gold /
   quickslot / persistence mutation while dead.
2. A matching confirm while still dead likewise fails closed with no frames and
   no mutation.
3. After `/restart_here` restores live HP, the same preview emits one
   `REFINE_INFORMATION_NEW`, and an injected succeeding roll (`75`) confirm
   emits the ordinary 5-frame success burst (`ITEM_DEL` material + material
   `QUICKSLOT_DEL` + source-cell `ITEM_SET(result_vnum)` + gold
   `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND` `RefineSuceeded <type>`),
   persisting the upgraded source (same instance id/cell), consumed materials,
   reduced gold, and cleared material quickslot beside recovered MaxHP while
   retaining the source item quickslot and unrelated skill quickslot.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same preview + injected-success confirm path likewise upgrades the
   source and persists beside recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorRefineConfirmInjectedSuccessSucceedsAfterRestartHere`
   - `TestGameSessionFlowPostFloorRefineConfirmInjectedSuccessSucceedsAfterRestartTown`

## Explicit non-goals

- inventing a death-specific refine packet family
- widening into catalyst / destroy / keep-on-fail / fail-result restart twins
  in this same commit (those siblings are already owned)
- mall / real `ITEM_GIVE` transfer / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorRefineConfirmInjectedSuccessSucceedsAfterRestart' -count=1
gofmt -w internal/minimal/post_floor_refine_confirm_injected_success_restart_recovery_test.go
git diff --check
```
