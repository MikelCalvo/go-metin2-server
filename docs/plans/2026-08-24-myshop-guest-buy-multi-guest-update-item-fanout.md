# MYSHOP guest buy multi-guest UPDATE_ITEM fan-out proof — 2026-08-24

## Why this exists

Guest private-shop buy already transfers live host stock/gold and emits buyer
`GC::SHOP UPDATE_ITEM(vnum=0)` for the sold display slot. Shared-world
`CommitMyShopGuestBuyStockClear` already fans that same sold-slot refresh to
other browsing guests, but the owned session proof only covers the buyer burst.
Without a second-guest proof, multi-client QA cannot trust that a still-browsing
watcher sees the sold slot clear while the host shop stays open.

The external oracle's `CShop::BroadcastUpdateItem` sends the sold-slot refresh
to every current guest map entry after PC-shop buy clears `pkItem`.

This slice is **multi-guest sold-slot UPDATE_ITEM fan-out proof only**. It does
not invent tax/empire multipliers, guest sell-into-PC-shop, shop-bag
consumption, or cube busy rejects.

## Contract to freeze / prove

1. Scope: one host with accepted open MYSHOP + one listed stock row; two visible
   same-map guests both successfully browse that host (`ON_CLICK` → `SHOP START`).
2. When guest A `SHOP BUY`s the listed `display_pos` successfully:
   - guest A still receives buyer `UPDATE_ITEM(vnum=0)` in the direct buy burst
   - guest B receives exactly one queued `GC::SHOP UPDATE_ITEM` for that
     `display_pos` with `vnum = 0` (and empty sold-slot companions) via
     shared-world fanout, with no inventory/gold mutation on guest B
   - host still receives inventory/gold refresh; host MYSHOP stays open
3. Guest B browse association stays open until explicit leave / host close /
   lifecycle teardown; a later guest B buy of the same slot fails sold-out /
   invalid with no further mutation.
4. Spec/QA/packet-matrix/roadmap name this beside the owned guest buy seam;
   tax/empire multipliers, guest sell-into-PC-shop, shop-bag consumption, and
   cube busy rejects stay deferred.

## Explicit non-goals

- no tax / empire `*3` price / shop-bag (`50200` / `71049`) consumption
- no guest sell into a private shop
- no cube busy rejects beyond naming them deferred
- no claim that private-shop trading is complete once multi-guest fan-out is proven

## Proof shape

1. Runtime/session: host opens MYSHOP → guest A and guest B browse → guest A
   buys → guest B flush receives one `UPDATE_ITEM(vnum=0)` for that display slot
   and guest B account/inventory/gold stay unchanged; guest B second buy of the
   sold slot is sold-out/invalid.
2. Docs/spec/QA name the fan-out beside owned guest buy; deferred seams stay deferred.

## Status

Implemented on `lane/items`: session proof owns second browsing guest
`UPDATE_ITEM(vnum=0)` fan-out after another guest buys, with no watcher
inventory/gold mutation and sold-out retry on the cleared slot. Tax/empire
multipliers, guest sell-into-PC-shop, shop-bag consumption, and cube busy
rejects stay deferred.
