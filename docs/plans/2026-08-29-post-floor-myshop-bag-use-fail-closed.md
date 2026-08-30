# Post-floor shop/silk-bag USE fail-closed — 2026-08-29

## Objective

Close the remaining private-shop setup twin after host-only `CG::MYSHOP` already
owns post-floor open denial: prove carried ordinary shop bag `50200` and silk bag
`71049` `ITEM_USE` / `/use_item` cannot emit `OpenPrivateShop` /
`MyShopPriceList` command chat (and cannot mutate inventory) while the selected
owner is still at the practice-mob retaliation `0`-HP floor, then recover via
`/restart_here` / `/restart_town`.

## Contract frozen by this slice

1. Drive the owner to the retaliation HP floor with a content-loaded practice mob.
2. Later carried shop-bag `ITEM_USE` and `/use_item <slot>` fail closed with:
   - no `CHAT_TYPE_COMMAND` `OpenPrivateShop`
   - no `MyShopPriceList` lines
   - no busy/armor INFO
   - no inventory / gold / persistence mutation
3. Later carried silk-bag `ITEM_USE` likewise fails closed with the same quiet
   denial before `/restart_town`.
4. After `/restart_here` restores live HP, shop-bag `ITEM_USE` emits only
   `OpenPrivateShop` and leaves the bag unconsumed.
5. After `/restart_town` restores live HP at the owned empire town position,
   shop-bag `ITEM_USE` emits only `OpenPrivateShop` and leaves the bag
   unconsumed; silk-bag `ITEM_USE` emits `MyShopPriceList 1 0` then
   `OpenPrivateShop` and leaves the bag unconsumed.
6. Spec/QA name the focused twins:
   - `TestGameSessionFlowPostFloorShopBagUseFailsClosed`
   - `TestGameSessionFlowPostFloorShopBagUseFailsClosedBeforeRestartTown`
   - `TestGameSessionFlowPostFloorSilkBagUseFailsClosedBeforeRestartTown`

## Explicit non-goals

- inventing a death-specific bag-use packet family
- changing busy-shell / armor INFO rejects for live owners
- inventing GD/DB `MYSHOP_PRICELIST_*` rematerialize
- changing already-owned host-only `CG::MYSHOP` post-floor open denial

## Validation

```bash
go test ./internal/minimal -run 'TestGameSessionFlowPostFloor(ShopBagUse|SilkBagUse|MyShopOpen|OpenCube|OpenSafebox)' -count=1
gofmt -w internal/minimal/player_death_busy_shell_open_guard_test.go
git diff --check
```
