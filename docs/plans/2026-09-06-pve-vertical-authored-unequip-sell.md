# PvE vertical authored unequip/sell — 2026-09-06

## Objective

Close the remaining honesty gap in
`TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn`: the
composed authoring fixture already equips turn-in `Wooden Sword` `11200` with
authored `equip_slot = weapon` and authors `shop_sell_price = 100`, and manual
QA already expects one packet `SHOP SELL` from an open merchant window, but the
automated proof stopped after equip plus a later merchant reopen.

## Why now

- Packet unequip and `SHOP SELL` success / `INVALID_POS` frames are already owned.
- Non-zero `shop_sell_price` is already the explicit per-unit sell credit
  (`100`, not the older `/5` + 3% tax derivation).
- `11200` has no `shop_buy_price`, so stripping `shop_sell_price` would fail
  closed instead of inventing a derived credit.
- Equipped items cannot be sold from carried slots; the sword must unequip first.

## Contract frozen by this slice

1. After `QuestGuide` re-unlock reopens the QA merchant, packet `SHOP SELL`
   carried slot `0` while the sword is still worn returns one self-only
   `GC::SHOP INVALID_POS`, leaves gold/equipment/inventory unchanged, and
   keeps the merchant window open.
2. Packet `ITEM_MOVE` from the authored weapon wear cell onto empty carried
   slot `0` emits `ITEM_DEL(equipment)` / carried `ITEM_SET` / self
   `CHARACTER_UPDATE` with weapon appearance part `0`.
3. Packet `SHOP SELL` of that unequipped last stack credits authored
   `shop_sell_price = 100` (`ITEM_DEL` then `PLAYER_POINT_CHANGE(POINT_GOLD)`),
   with no extra `GC::SHOP OK`. Live and persisted gold increase by `100`;
   inventory and equipment are empty.
4. Runtime unit: `MerchantSellCredit` for `11200` with `shop_sell_price = 100`
   returns `100`; the same template without `shop_sell_price` / `shop_buy_price`
   fails closed.

## What this is not yet

- warehouse/safebox mutation in the same composed proof
- pack-member combat or foreign-map warp coverage
- claiming the whole merchant/item system is now template-complete

## Verification

```bash
gofmt -w internal/minimal/pve_vertical_authoring_test.go internal/player/runtime_inventory_test.go
go test ./internal/player ./internal/minimal -count=1 \
  -run 'TestMerchantSellCreditPrefersAuthoredWoodenSwordShopSellPrice|TestPveVerticalAuthoringBundleClosesGuideUnlockKillCreditAndTurnIn$'
git diff --check
```
