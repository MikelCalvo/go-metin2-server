# Post-floor silk-bag USE `/restart_here` recovery — 2026-08-30

## Objective

Close the remaining recovery twin after
`TestGameSessionFlowPostFloorSilkBagUseFailsClosedBeforeRestartTown` already
proved silk-bag `71049` deny → `/restart_town` recovery: prove the same
post-floor quiet denial also recovers through `/restart_here` so both bag
companions match the shop-bag / host-only `MYSHOP` restart matrix.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob.
2. Later carried silk-bag `ITEM_USE` and `/use_item <slot>` fail closed with:
   - no `CHAT_TYPE_COMMAND` `OpenPrivateShop`
   - no `MyShopPriceList` lines
   - no busy/armor INFO
   - no inventory / gold / persistence mutation
3. After `/restart_here` restores live HP, silk-bag `ITEM_USE` emits
   `MyShopPriceList 1 0` then `OpenPrivateShop` and leaves the bag unconsumed.
4. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorSilkBagUseFailsClosed`
   - `TestGameSessionFlowPostFloorSilkBagUseFailsClosedBeforeRestartTown`
   - `TestGameSessionFlowPostFloorShopBagUseFailsClosed`
   - `TestGameSessionFlowPostFloorShopBagUseFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific bag-use packet family
- changing busy-shell / armor INFO rejects for live owners
- inventing GD/DB `MYSHOP_PRICELIST_*` rematerialize
- changing already-owned shop-bag or host-only `CG::MYSHOP` post-floor denial

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloor(ShopBagUse|SilkBagUse)' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
