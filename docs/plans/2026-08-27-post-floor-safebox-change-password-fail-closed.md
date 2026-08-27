# Post-floor warehouse `/safebox_change_password` fail-closed — 2026-08-27

## Objective

Close the remaining warehouse durable-mutation twin after post-floor
`/open_safebox` and pending `/safebox_password` already own open denials: prove
`/safebox_change_password` cannot mutate durable warehouse password (and cannot
emit success/wrong-password chat) while the selected owner is still at the
practice-mob retaliation `0`-HP floor.

## Contract frozen by this slice

1. After immediate practice-mob retaliation reaches owner HP `0`, a later
   `/safebox_change_password <old> <new>` attempt fails closed with:
   - no self info-chat (`The warehouse password has been changed.` /
     `You have entered the wrong password.`)
   - no durable password mutation in the same-account safebox FileStore
   - no inventory / gold / presentation / persistence mutation
2. After `/restart_here` restores live HP, the same change-password command
   succeeds normally and persists the new durable password.
3. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorSafeboxChangePasswordFailsClosed`

## Explicit non-goals

- inventing a death-specific safebox packet family
- widening `/safebox_money_save` / `/safebox_money_withdraw` beyond the already
  live floor gates (those remain separate twins if still unproven)
- changing already-owned open-presentation floor **close** / restart exchange
  recovery twins

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloor(OpenSafebox|SafeboxPasswordOpen|SafeboxChangePassword|OpenCube|MyShopOpen|RefinePreviewOpen|ExchangeStart)' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
