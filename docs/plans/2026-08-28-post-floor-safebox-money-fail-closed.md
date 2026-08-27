# Post-floor warehouse `/safebox_money_*` fail-closed — 2026-08-28

## Objective

Close the remaining warehouse durable-mutation twin after post-floor
`/open_safebox`, pending `/safebox_password`, and `/safebox_change_password`
already own open/password denials: prove `/safebox_money_save` and
`/safebox_money_withdraw` cannot mutate carried gold or durable warehouse
money (and cannot emit gold `PLAYER_POINT_CHANGE` / `SAFEBOX_MONEY_CHANGE`)
while the selected owner is still at the practice-mob retaliation `0`-HP floor.

## Contract frozen by this slice

1. Seed warehouse money while alive (`/open_safebox` + `/safebox_money_save`),
   close presentation, then drive the owner to the retaliation HP floor.
2. Later `/safebox_money_save <amount>` and `/safebox_money_withdraw <amount>`
   attempts fail closed with:
   - no gold `PLAYER_POINT_CHANGE`
   - no `SAFEBOX_MONEY_CHANGE`
   - no carried-gold mutation
   - no durable warehouse-money mutation
3. After `/restart_here` restores live HP and a fresh `/open_safebox` rematerializes
   the pre-floor warehouse total, `/safebox_money_withdraw` succeeds normally.
4. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorSafeboxMoneyFailsClosed`

## Explicit non-goals

- inventing a death-specific safebox money packet family
- widening TMP4 CG `SAFEBOX_MONEY` request ownership (still deferred)
- changing already-owned open-presentation floor **close** / restart exchange
  recovery twins

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloor(OpenSafebox|SafeboxPasswordOpen|SafeboxChangePassword|SafeboxMoney|OpenCube|MyShopOpen|RefinePreviewOpen|ExchangeStart)' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
