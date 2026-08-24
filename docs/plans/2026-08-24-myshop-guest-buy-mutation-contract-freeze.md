# MYSHOP guest private-shop buy mutation contract freeze — 2026-08-24

## Why this exists

Guest browse open + leave/`SHOP END` are now owned: a visible peer can `ON_CLICK` an already-open MYSHOP host, receive one guest-only `GC::SHOP START` stock table, and leave with one guest-only `GC::SHOP END`. Without a buy freeze, the next RED would invent whether buy reuses merchant `CG::SHOP BUY`, how live host stock is transferred, whether empire tax/price multipliers apply, whether sold slots emit `UPDATE_ITEM`, and how distance / sold-out / inventory-full failures surface.

The external oracle routes guest buy through the same `CG::SHOP BUY` family while the guest still has `GetShop()` / `GetShopOwner()` from `AddGuest`, then `CShop::Buy` for a PC shop moves the live host item into the buyer, debits buyer gold, credits host gold, clears the shop slot, and `BroadcastUpdateItem`s the sold slot to remaining guests. Guest sell into a PC shop is rejected.

This freeze is **first guest private-shop buy mutation only**. It does not invent guest sell-into-PC-shop, tax/empire multipliers, shop-bag consumption, or cube busy rejects.

## Oracle summary (behavior reference only)

- Ingress: `CG::SHOP` / `BUY` with display/catalog slot byte while guest still has shop association from browse
- Manager requires `GetShop()` + `GetShopOwner()`; `ApproxDistance > 2000` rejects with info-chat (`You are too far away from the shop to buy something.`) and no shop error frame
- `CShop::Buy` for PC shop:
  - missing guest map → `END`
  - invalid pos → `INVALID_POS`
  - missing / wrong-owner live item → fail closed (no success)
  - insufficient gold → `NOT_ENOUGH_MONEY`
  - no empty inventory placement → `INVENTORY_FULL`
  - success: debit buyer gold, remove item from host, place into buyer inventory, clear shop slot, `BroadcastUpdateItem(pos)` with `vnum = 0` to guests, credit host gold, return `OK`
- Shop manager emits bare `GC::SHOP` error subheaders only when result is not `OK`; success does **not** emit `GC::SHOP OK`
- PC-shop `Sell` returns immediately (guest cannot sell into private shop)
- Empire `*3` price applies only when guest was marked other-empire; PC `AddGuest` passes `false`, so private-shop browse/buy uses listed price
- Tax / `personal_shop` event flag may reduce host credit in some locales; bootstrap freezes tax as deferred (`iVal = 0`, full listed price credited)

## Contract to freeze (before RED)

Own the first guest private-shop buy path on top of already-owned guest browse open/leave:

1. Scope: guest selected character already in `GAME`, above the zero-HP floor, with `activeGuestMyShopHostVID != 0` from a successful browse against a still-open host MYSHOP (shared-world busy bit + remembered stock listing).
2. Ingress: valid `CG::SHOP BUY` (`HandleShopBuy`) may become the guest-buy path when guest browse is open and no same-socket NPC merchant window is open. Active merchant buy keeps the already-owned merchant path. No-browse / no-merchant `BUY` stays fail-closed with no frames.
3. Distance gate: when `ApproxDistance` from guest live position to host live position is `> 2000` on the same effective map (or host is no longer a live same-map peer), reject with one self-only `CHAT_TYPE_INFO` `You are too far away from the shop to buy something.`; no shop error frame, no inventory/gold mutation, browse flag stays open.
4. Stock resolution (fail closed before mutation):
   - `CatalogSlot` / display pos must address one remembered host stock row whose `display_pos` matches
   - that row must still resolve to exactly one live host carried cell matching remembered source cell + `vnum` + `count`, unlocked / unequipped / well-formed, still owned by the host
   - host must still have open MYSHOP with that remembered listing; otherwise treat as sold-out / invalid and fail closed
   - resolved template must still not author `anti_give` / `anti_myshop` (open already gated these; drift fails closed)
5. Gold / capacity gates:
   - guest live gold must be `>=` remembered listed `price` (no empire `*3` for private shop); else one bare self-only `GC::SHOP NOT_ENOUGH_MONEY`
   - guest must have a valid carried placement for the transferred stack under the already-owned merchant-buy / exchange-finalize placement rules (merge into compatible unlocked stacks first, else a free carried cell); else one bare self-only `GC::SHOP INVENTORY_FULL`
   - host live gold + listed price must fit the owned signed gold carrier (`1<<31-1`); overflow fails closed with no frames / no mutation (do not invent a new chat string in this slice)
   - invalid / empty / already-sold display slot → one bare self-only `GC::SHOP INVALID_POS` or `GC::SHOP SOLD_OUT` / `SOLDOUT` as appropriate (`INVALID_POS` for out-of-range / never-listed; `SOLD_OUT` when the remembered slot is already cleared or live stock no longer matches)
6. Success mutation (atomic live + persist both selected-character snapshots):
   - debit guest gold by the remembered listed `price`
   - remove the matched host carried stack (whole listed count) and sync any host source-item quickslots that pointed at that cell through the already-owned quickslot deletion ordering
   - place the transferred item identity into the guest via the placement rules above (preserve item identity / sockets / attributes / count)
   - credit host gold by the full listed `price` (tax deferred → `iVal = 0`)
   - clear that remembered host stock row / display slot so a later buy of the same slot fails sold-out
   - persist guest and host account snapshots; either write failure rolls both sides back fail-closed with no frames
7. Success frames (no bare `GC::SHOP OK`):
   - guest: self-only inventory refresh (`ITEM_SET` / `ITEM_UPDATE` as already owned by merchant buy) + gold `PLAYER_POINT_CHANGE`
   - host: queued inventory refresh for the removed cell (`ITEM_DEL` / `ITEM_UPDATE`) + any `GC::QUICKSLOT_DEL` required by source-cell removal + gold `PLAYER_POINT_CHANGE`
   - every remaining guest still browsing that host (including the buyer, while browse stays open): one `GC::SHOP UPDATE_ITEM` for the sold `display_pos` with `vnum = 0` (codec already owned); browse flags stay open until explicit leave / host close / lifecycle teardown
8. Explicit non-mutation / fail-closed:
   - guest `SHOP SELL` / `SELL2` while browsing a private shop stay fail-closed with no frames (oracle rejects PC-shop sell)
   - do not close host MYSHOP on buy; do not empty-sign; do not invent bag consumption / pricelist / polymorph
   - do not apply empire price multipliers or tax deductions in this slice
   - cube busy rejects stay deferred
9. Spec/QA/packet-matrix name guest private-shop buy beside owned browse open/leave; guest sell-into-PC-shop, tax/empire multipliers, and cube busy rejects stay deferred.

## Locale / wording note

Distance reject reuses the English locale string already present in the external oracle locale table: `You are too far away from the shop to buy something.` Shop error frames reuse the already-owned bare merchant `NOT_ENOUGH_MONEY` / `INVENTORY_FULL` / `INVALID_POS` / `SOLD_OUT` companions. Do not invent authored template buy-reject text for private-shop buy in this slice.

## Explicit non-goals

- no guest sell into a private shop
- no empire `*3` price or locale tax on host credit
- no shop-bag (`50200` / `71049`) consumption, DB pricelist, or polymorph / mount teardown
- no dragon-soul / size-grid inventory beyond already-owned carried placement
- no cube busy rejects beyond naming them deferred
- no claim that private-shop trading is complete once one buy lands (host restock UI, multi-guest races beyond fail-closed sold-out, and mall remain out of scope)

## Proof shape for the implementation slice

1. Runtime/session: host opens MYSHOP with one listed carried row + price → guest browses → guest `SHOP BUY` that display slot debits guest gold, removes host stack, grants guest item, credits host gold, persists both, emits guest/host inventory+gold refreshes, and emits guest `UPDATE_ITEM(vnum=0)` for that slot; second buy of same slot is sold-out / invalid with no further mutation.
2. Negatives: distance `> 2000` → distance info-chat only; insufficient gold → `NOT_ENOUGH_MONEY`; full inventory → `INVENTORY_FULL`; no browse flag → silent; `SELL` while browsing private shop → silent.
3. Docs/spec/QA name the buy seam; tax/empire multipliers / guest sell-into-PC-shop / cube busy rejects stay deferred.

## Status

Docs-first freeze on `lane/items`. Implementation / RED intentionally deferred to the next items-lane slice.
