# Post-floor merchant `SHOP SELL2` restart recovery — 2026-08-31

## Objective

Close the remaining partial-stack merchant-sell recovery twin after packet
`SHOP SELL2` already owned quiet post-floor denial and whole-stack `SHOP SELL`
restart recovery landed on the items lane: prove `/restart_here` /
`/restart_town` restore a usable live owner that can freshly reopen a merchant
and complete an ordinary partial-stack `SHOP SELL2`.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a merchant window is already open and the owner carries one
   sellable stack (`count = 3`) plus an item quickslot on that cell.
2. Later packet `SHOP SELL2` (`count = 2`) fails closed with no frames and no
   inventory / gold / quickslot / persistence mutation.
3. After `/restart_here` restores live HP, wait past the owned static-actor
   interaction cooldown, then a fresh merchant `INTERACT` emits ordinary
   `GC::SHOP START`, and the same `SHOP SELL2` emits `ITEM_UPDATE` (remaining
   count `1`) + gold `PLAYER_POINT_CHANGE` (`amount = 2`, `value = 127`) and
   persists the remaining stack / credited gold / unchanged quickslots beside
   recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position,
   the same reopen + partial sell path against a destination-map merchant
   likewise succeeds and persists beside recovered MaxHP and the town-return
   coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorShopSell2FailsClosed`
   - `TestGameSessionFlowPostFloorShopSell2FailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific merchant packet family
- widening into `SHOP BUY`, whole-stack `SHOP SELL` (already owned elsewhere),
  safebox check-in success, or refine confirm twins in this same commit
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorShopSell2FailsClosed' -count=1
gofmt -w internal/minimal/post_floor_shop_sell2_restart_recovery_test.go
git diff --check
```
