# MYSHOP bag USE → MyShopPriceList + OpenPrivateShop — 2026-08-28

## Objective

Freeze then implement the client-visible silk/shop-bag `ITEM_USE` /
`/use_item` companion after MYSHOP open auto-potion `socket0` deactivate: using
carried ordinary shop bag `50200` or silk bag `71049` opens the private-shop
setup UI via self-only `CHAT_TYPE_COMMAND` frames, and the first silk USE in a
session rematerializes remembered unit prices as `MyShopPriceList` command
lines — without inventing GD/DB `MYSHOP_PRICELIST_*` packets, bag-missing INFO,
quest-running open blocks, zone tables, or shopkeeper polymorph.

## Why this exists

Track C's live next line prefers `MYSHOP_PRICELIST` / quest-running /
bag-missing INFO / refine keep-grade only with client-visible evidence;
`LESS_GOLD` auto-cancel stays out. The external oracle's silk USE path is the
client-visible pricelist companion (`MyShopPriceList <vnum> <unitPrice>` then
`OpenPrivateShop`); ordinary shop-bag USE only sends `OpenPrivateShop`. Pure
GD/DB rematerialize on `CG::MYSHOP` silk success emits no client frames, so it
is not the smallest useful seam. Bag-missing INFO is oracle-silent; quest
`IsRunning` has no bootstrap script-running substrate.

## Contract to freeze (before / with GREEN)

1. **Ingress**: same-socket packet `ITEM_USE` or slash `/use_item <slot>` while
   selected character is in `GAME`, above the bootstrap zero-HP floor, targeting
   one carried unlocked unequipped well-formed live cell whose `vnum` is
   `50200` or `71049` and whose template resolves through the loaded itemstore
   (no `use_effect` / point consumable path required; this is a dedicated bag
   USE seam).
2. **Non-consume**: success does **not** debit inventory, quickslots, gold, or
   persistence for either bag vnum.
3. **Busy gate** (before armor / success): when the requester already has an
   open same-socket merchant window, safebox presentation, refine dialog,
   private-shop presentation, lab cube presentation, or bootstrap exchange
   shell, emit one self-only `CHAT_TYPE_INFO`
   `You cannot use a shop bag or silk bag while another trade window is open.`,
   leave those shells open, and perform no inventory/command mutation.
4. **Armor gate**: when `EquipmentSlotBody` is occupied, emit the already-owned
   `You must unequip your armor to open a private shop.` INFO; no command
   frames; no mutation.
5. **Ordinary shop bag (`50200`) success**: one self-only
   `CHAT_TYPE_COMMAND` `OpenPrivateShop`.
6. **Silk bag (`71049`) success**:
   - **First silk USE in this same-socket session** (or after process-local
     remembered prices were never rematerialized yet): emit zero-or-more
     self-only `CHAT_TYPE_COMMAND` `MyShopPriceList <vnum> <unitPrice>` lines,
     then `OpenPrivateShop`.
     - When the session has no remembered unit prices, emit exactly one dummy
       `MyShopPriceList 1 0` before `OpenPrivateShop`.
     - When the session remembers unit prices from a prior accepted silk-path
       `CG::MYSHOP` open in this process/session, emit one
       `MyShopPriceList` per remembered `(vnum, unitPrice)` in deterministic
       ascending `vnum` order, then `OpenPrivateShop`.
   - **Later silk USE in the same session** after that first rematerialize:
     emit only `OpenPrivateShop` (no repeated price-list dump).
7. **Remembered unit prices** (process-local / same-socket only): on accepted
   silk-path `CG::MYSHOP` open success, replace the remembered map with
   `unitPrice = listed_price / listed_count` per distinct listed stock `vnum`
   (integer division). Do **not** invent durable GD/DB writes or SQL. Ordinary
   `50200` open does not update the remembered map. Reconnect / new session still starts process-local rematerialize pending;
     durable account FileStore `myshop_unit_prices` rematerialize across reconnect /
     restart is owned by `docs/plans/2026-08-29-myshop-unit-prices-durable-filestore.md`.
8. **Ordering / echo**: bag USE success emits only the command-chat frames
   above (no ordinary consumable `ITEM_USE` echo / point / `ITEM_UPDATE` burst).
   Packet and slash ingress share the same command burst.
9. **Fail-closed silent** (no frames / no mutation): locked / equipped /
   duplicate-slot / missing template / wrong window / displayed-exchange source
   cell / zero-HP / no selected character / already-open MYSHOP host mutation
   lock for non-bag USE stays as already owned (bag USE itself uses the busy
   INFO above when MYSHOP is open).
10. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP / item-use once
    GREEN. Do **not** invent zone `IS_BOTARYABLE` tables, Canada locale gates,
    GD packets, bag-missing INFO, quest-running, or polymorph in this slice.

## Locale / wording note

Reuse owned armor INFO. Busy INFO is the English bootstrap string above (oracle
Korean trade-window prevent). Command payloads are exact ASCII
`OpenPrivateShop` and `MyShopPriceList %u %u`. Do not copy oracle source
comments or Korean keys into runtime code.

## Explicit non-goals

- GD/DB `MYSHOP_PRICELIST_REQ` / `UPDATE` / `RES` packets / SQL myshop_pricelist tables
  (account FileStore durable unit prices are a separate bootstrap stand-in)
- bag-missing INFO (oracle ordinary-bag miss stays silent)
- quest-running open block (`PC::IsRunning`)
- zone / botaryable-map gate
- shopkeeper polymorph / horse / mount teardown
- Canada banword bypass / DB banword reload
- guest-buy tax / empire `*3`
- refine keep-grade / catalysts
- `LESS_GOLD` exchange auto-cancel
- SQL instance-socket columns / import backfill

## Proof shape

1. Runtime/session: carried `71049`, no remembered prices → `/use_item` /
   `ITEM_USE` emits `MyShopPriceList 1 0` then `OpenPrivateShop`; inventory /
   gold / quickslots unchanged.
2. Runtime/session: silk-path `CG::MYSHOP` open with listed stock → later silk
   USE rematerializes remembered unit prices (sorted) then `OpenPrivateShop`;
   second silk USE in-session emits only `OpenPrivateShop`.
3. Runtime/session: carried `50200` USE emits only `OpenPrivateShop`; no
   consume.
4. Negatives: body armor → owned armor INFO; merchant/safebox/refine/myshop/
   cube/exchange busy → bag-use busy INFO; locked silk → silent; ordinary
   consumable USE and silk `CG::MYSHOP` consume-skip / auto-potion deactivate
   stay GREEN.

## Status

GREEN on `lane/items`: silk/shop-bag `ITEM_USE` / `/use_item` emits
`MyShopPriceList` + `OpenPrivateShop` command chat with process-local remembered
unit prices from silk-path `CG::MYSHOP` open; armor / busy-shell rejects and
non-consume proofs are owned. GD/DB `MYSHOP_PRICELIST`, quest-running,
bag-missing INFO, shopkeeper polymorph, and refine keep-grade stay deferred;
account FileStore durable unit-price rematerialize is owned separately
(`docs/plans/2026-08-29-myshop-unit-prices-durable-filestore.md`).
`LESS_GOLD` auto-cancel stays out.
