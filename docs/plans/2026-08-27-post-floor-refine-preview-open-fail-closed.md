# Post-floor refine-dialog preview open fail-closed — 2026-08-27

## Objective

Close the remaining combat-lane busy-shell **open** twin after safebox / cube /
MYSHOP / exchange already owned post-floor open denials: prove a refineable
carried item cannot open `REFINE_INFORMATION_NEW` / set refine busy while the
selected owner is still at the practice-mob retaliation `0`-HP floor.

## Contract frozen by this slice

1. After immediate practice-mob retaliation reaches owner HP `0`, a later
   refineable `REFINE` preview request fails closed with:
   - no `REFINE_INFORMATION_NEW`
   - no same-socket refine-dialog busy flag
   - no inventory / gold / quickslot / persistence mutation
2. This is distinct from the already-owned
   `TestGameSessionFlowPostFloorItemRefineFailsClosedBeforeRejectFeedback`
   reject-feedback path (non-refineable + `refine_reject_message`).
3. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorRefinePreviewOpenFailsClosed`

## Explicit non-goals

- inventing a death-specific refine packet family
- widening refine confirm/catalyst/keep-grade behavior
- changing already-owned floor **close** / restart exchange-recovery twins

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloor(OpenSafebox|OpenCube|MyShopOpen|RefinePreviewOpen|ExchangeStart)' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
