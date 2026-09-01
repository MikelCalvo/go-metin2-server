# Post-floor merchant `SHOP BUY` restart recovery — 2026-09-01

## Objective

Close the remaining merchant-buy recovery twin after packet `SHOP BUY` already
owned quiet post-floor denial: prove `/restart_here` / `/restart_town` restore a
usable live owner that can freshly reopen a merchant and complete an ordinary
catalog-slot `SHOP BUY`.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice
   mob while a merchant window is already open and the owner holds enough gold
   for catalog slot `0` (`27001` / price `50`) with empty carried inventory.
2. Later packet `SHOP BUY` fails closed with no frames and no inventory / gold /
   persistence mutation.
3. After `/restart_here` restores live HP, wait past the owned static-actor
   interaction cooldown, then a fresh merchant `INTERACT` emits ordinary
   `GC::SHOP START`, and the same `SHOP BUY` emits the ordinary one-frame
   inventory `ITEM_SET` for catalog slot `0` and persists gold debit + bought
   stack beside recovered MaxHP.
4. After `/restart_town` restores live HP at the owned empire town position, the
   same reopen + buy path against a destination-map merchant likewise succeeds
   and persists beside recovered MaxHP and the town-return coordinates.
5. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorShopBuyFailsClosed`
   - `TestGameSessionFlowPostFloorShopBuyFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific merchant packet family
- widening into safebox check-in success, refine confirm, or mall twins in this
  same commit
- refine catalysts / mall / GD-DB `MYSHOP_PRICELIST`

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloorShopBuyFailsClosed' -count=1
gofmt -w internal/minimal/post_floor_shop_buy_restart_recovery_test.go
git diff --check
```
