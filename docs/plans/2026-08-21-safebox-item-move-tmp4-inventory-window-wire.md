# Safebox Item-Move TMP4 Inventory-Window Wire — 2026-08-21

## Objective

Accept the real TMP4 client `SAFEBOX_ITEM_MOVE` wire, which packs both source and destination `TItemPos` with the inventory window type and safebox-slot cell bytes, so manual-client drag moves inside an open `/open_safebox` presentation stop failing closed.

## Contract owned by this slice

1. `CG::SAFEBOX_ITEM_MOVE` remains accepted only while `/open_safebox` is already open and the selected character is above the zero-HP floor.
2. Source and destination cells remain distinct in-range safebox slot indices inside `size * 5` capacity.
3. Window-type policy for those packed positions:
   - `WindowInventory` (TMP4 client send helper) and `WindowSafebox` (explicit/test tooling) are both accepted for either position;
   - cells are always interpreted as same-session safebox slot indices, never as carried-inventory cells;
   - `WindowMall`, `WindowEquipment`, reserved, and any other window stay fail-closed with no frames.
4. Whole-stack relocate / compatible merge and partial-count split / compatible partial merge keep the already-owned mutation, frame, merchant/exchange auto-close, and no-inventory/quickslot/gold/account-persistence contracts.
5. Spec/QA name the TMP4 inventory-window wire as the primary accepted client path beside the explicit `WindowSafebox` tooling path.

## What this is not yet

- password / DB load and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / money / mall
- partner-side open player-shop / cube busy-window exchange rejects
- multi-cell item-size grid packing beyond bootstrap 1-cell occupancy

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SafeboxItemMove' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep money / password / durable persistence deferred.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
