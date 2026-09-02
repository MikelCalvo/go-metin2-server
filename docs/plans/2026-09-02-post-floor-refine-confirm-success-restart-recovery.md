# Post-floor `probability = 100` refine confirm success restart recovery — 2026-09-02

## Objective

Close the remaining accepted refine recovery twin after post-floor refine
preview reopen and non-refineable reject restart twins already landed: prove
`/restart_here` / `/restart_town` restore a usable live owner that can freshly
preview a refineable carried item and complete a matching `probability = 100`
confirm with the ordinary success burst.

## Contract frozen by this slice

1. Seed one refineable carried source plus one fully-consumed material stack,
   drive the owner to the retaliation HP floor, then later preview `REFINE`
   fails closed with no `REFINE_INFORMATION_NEW` and no inventory / gold /
   quickslot / persistence mutation while dead.
2. A matching confirm while still dead likewise fails closed with no frames and
   no mutation.
3. After `/restart_here` restores live HP, the same preview emits one
   `REFINE_INFORMATION_NEW`, and the matching confirm emits the ordinary
   4-frame success burst (`ITEM_DEL` material + result `ITEM_SET` + gold
   `PLAYER_POINT_CHANGE` + `CHAT_TYPE_COMMAND` `RefineSuceeded <type>`),
   persisting result `vnum`, consumed materials, and reduced gold beside
   recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same preview + confirm path likewise succeeds and persists beside
   recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorRefineConfirmSucceedsAfterRestartHere`
   - `TestGameSessionFlowPostFloorRefineConfirmSucceedsAfterRestartTown`

## Explicit non-goals

- inventing a death-specific refine packet family
- widening into catalyst / `probability` `0` / `1..99` / keep-on-fail /
  fail-result restart twins in this same commit
- mall / real `ITEM_GIVE` transfer / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorRefineConfirmSucceedsAfterRestart' -count=1
gofmt -w internal/minimal/post_floor_refine_confirm_success_restart_recovery_test.go
git diff --check
```
