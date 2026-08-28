# Post-floor warehouse `/safebox_money_*` `/restart_town` recovery — 2026-08-28

## Objective

Close the remaining recovery twin after
`TestGameSessionFlowPostFloorSafeboxMoneyFailsClosed` already proved
`/restart_here` rematerializes durable warehouse money: prove the same
post-floor deny → recover path also works through `/restart_town` into the
owned empire town-return position.

## Contract frozen by this slice

1. Seed warehouse money while alive (`/open_safebox` + `/safebox_money_save`),
   close presentation, then drive the owner to the retaliation HP floor.
2. Later `/safebox_money_save <amount>` and `/safebox_money_withdraw <amount>`
   attempts fail closed with:
   - no gold `PLAYER_POINT_CHANGE`
   - no `SAFEBOX_MONEY_CHANGE`
   - no carried-gold mutation
   - no durable warehouse-money mutation
3. After `/restart_town` restores live HP at the empire town-return position
   and a fresh `/open_safebox` rematerializes the pre-floor warehouse total,
   `/safebox_money_withdraw` succeeds normally and persists town map/coords.
4. Spec/QA name the focused twin:
   - `TestGameSessionFlowPostFloorSafeboxMoneyFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific safebox money packet family
- widening TMP4 CG `SAFEBOX_MONEY` request ownership (still deferred)
- destination-peer exchange recovery (already owned by open-safebox town twins)

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorSafeboxMoneyFailsClosed' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
