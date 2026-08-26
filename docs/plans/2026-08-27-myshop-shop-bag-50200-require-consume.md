# MYSHOP shop-bag (`50200`) require-and-consume gate — 2026-08-27

## Objective

Close the remaining host-only `CG::MYSHOP` open economy hole after armor / cash-item /
locked / gold-overflow reject chat: require one carried unlocked ordinary shop bag
(`vnum = 50200`) and consume it on accepted open, matching the external
`OpenMyShop` bag branch without inventing the silk-bag (`71049`) pricelist path.

## Why this exists

Accepted open currently remembers stock and emits `GC::SHOP_SIGN` with **no bag
cost**. The oracle aborts silently when neither `71049` nor `50200` is present,
and otherwise removes one `50200`. Manual QA can therefore open a private shop
for free. Track C after MYSHOP open reject-chat hardening prefers this mutation
seam over banword / quest-running / authored `myshop_reject_message` display
work.

## Contract to freeze (before / with GREEN)

1. **Gate ordering** on the accepted decode path (unchanged earlier gates):
   death-floor / no-selected / no-shared → silent;
   empty sign / zero count → silent;
   busy shells → owned busy info-chat;
   body-armor → armor chat;
   stock structural fails → silent;
   `anti_give|anti_myshop` → cash-item chat;
   locked stock → locked chat;
   gold overflow → gold chat;
   **then shop-bag**;
   success → consume bag + persist + `SHOP_SIGN`.
2. **Bag success**: host has at least one carried, unlocked, unequipped cell with
   `vnum = 50200` whose cell is **not** also listed in this open's stock table.
   Consume exactly `1` from the lowest such slot (ascending cell order):
   - count becomes `count-1` when `count > 1`, else the cell is removed
   - persist inventory + quickslots with the open
   - emit self-only removal refresh (`ITEM_UPDATE` or `ITEM_DEL`) before
     `GC::SHOP_SIGN`
   - when the cell is emptied, also emit already-owned item-removal
     `GC::QUICKSLOT_DEL` for bindings on that cell
   - then set the open/busy flag and emit the owned live `SHOP_SIGN` (+ peer
     around-broadcast as already owned)
3. **Bag missing / only listed / locked-only**: silent fail-closed
   (`Accepted: false`, no frames, no open flag, no inventory/gold mutation).
   Do **not** invent bag-missing INFO chat in this bootstrap slice (oracle
   `return` with no chat on the ordinary-bag miss path).
4. **Persist failure** after live bag debit rolls back live inventory/quickslots
   fail-closed with no frames and no open.
5. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open; `71049`
   silk-bag / DB pricelist, banword, quest-running, authored
   `myshop_reject_message`, polymorph/mount teardown stay deferred.

## Explicit non-goals

- silk-bag `71049` consume-skip + `MYSHOP_PRICELIST` DB packets
- banword / Canada locale sign filtering
- quest-running open block
- authored `myshop_reject_message`
- equipped-stock INFO chat beyond the existing silent missing-cell path
- polymorph / horse / mount teardown on open
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Runtime/session: one carried `50200` → open emits bag `ITEM_DEL` (or
   `ITEM_UPDATE`) then one `SHOP_SIGN`; account persists bag debit; second open
   without another bag stays silent.
2. Runtime/session: no `50200` → silent, no `SHOP_SIGN`, inventory unchanged.
3. Runtime/session: only bag cell is also listed stock → silent (do not consume
   listed stock).
4. Negative: armor / cash / locked / gold chats still win **before** bag gate.

## Status

Implemented on `lane/items` together with this freeze.
