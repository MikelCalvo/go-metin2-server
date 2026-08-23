# Host-only MYSHOP empty-sign close companion implementation — 2026-08-24

## Objective

Land the already-frozen host-only empty-sign `GC::SHOP_SIGN` clear/close companion so accepted private-shop open no longer tears down silently.

## Contract owned

1. While `hasActiveMyShopOpen` is set, lifecycle teardown (`/phase_select` / `/quit` / `/logout`), practice-mob floor close, and exact-position transfer/warp rebootstrap clear the busy flag and emit one self-only empty-sign `GC::SHOP_SIGN` with host VID.
2. Lab `/close_myshop` reuses the same helper and stays silent when already closed.
3. Ordering: merchant `GC::SHOP END` before empty-sign `SHOP_SIGN` before exchange `END` when those shells close together.
4. Inventory/gold unchanged; guest browse/buy and partner player-shop/cube exchange busy rejects stay deferred.

## Proof

- `go test ./internal/minimal -run 'TestGameRuntimeMyShop' -count=1`
- lifecycle open→`/quit|/logout|/phase_select` empty-sign + lifecycle frame
- `/close_myshop` empty-sign, already-closed silent, later lifecycle emits no second empty sign

## Status

Shipped on `lane/items`.
