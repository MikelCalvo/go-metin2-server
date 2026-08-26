# Open MYSHOP host item-mutation lock — 2026-08-24

## Objective

Freeze then implement the first open-private-shop host item-mutation fail-closed seam so an accepted `CG::MYSHOP` busy flag blocks host inventory/equipment/use/drop/pickup/give/safebox/refine mutations the same way the external oracle's `CanHandleItem()` rejects item handling while `GetMyShop()` is live.

## Why this exists

Host-only accepted private-shop open already remembers `hasActiveMyShopOpen`, emits self-only `GC::SHOP_SIGN`, and rejects exchange START/ACCEPT/commit. Stock remains carried until a later buy/sell slice owns transfer, so without a mutation lock the host can still use/move/drop/equip listed (or any) carried cells while the shop busy flag is set. That invents unsafe bootstrap behavior and diverges from the oracle `CanHandleItem` gate.

This freeze is **host item-mutation fail-closed while MYSHOP is open**. It does not invent guest browse/buy, peer `SHOP_SIGN` around-broadcast, stock removal, or cube busy rejects.

## Contract to freeze (before RED)

While the same-socket private-shop open/busy flag is set:

1. Fail closed with no frames / no inventory / equipment / quickslot / gold / ground / safebox / refine / exchange mutation for:
   - packet `ITEM_USE` / slash `/use_item`
   - packet `ITEM_USE_TO_ITEM`
   - packet `ITEM_MOVE` (carried↔carried, carried↔equipment, occupied-wear swap/replace)
   - slash `/inventory_move` / `/equip_item` / `/unequip_item`
   - packet `ITEM_DROP` / `ITEM_DROP2` (including gold-drop branches)
   - packet `ITEM_PICKUP`
   - packet `ITEM_GIVE` (including authored `give_reject_message` feedback paths)
   - open-presentation `SAFEBOX_CHECKIN` / `SAFEBOX_CHECKOUT` / `SAFEBOX_ITEM_MOVE`
   - refine preview (`REFINE` → `REFINE_INFORMATION_NEW`) and matching confirm (`type != 255`); `type = 255` cancel of an already-open refine dialog stays allowed only when no MYSHOP is open — when MYSHOP is open, refine preview/confirm stay fail-closed; cancel remains a no-op clear only if a refine dialog somehow exists, but MYSHOP open already prevents opening refine
2. Do **not** invent a new info-chat string in this bootstrap slice; silent no-frame consume matches other busy-window item-lock style (exchange displayed-cell locks) rather than inventing private-shop-only chat.
3. Private-shop open/busy flag and self-only live/empty `GC::SHOP_SIGN` path stay unchanged by these rejects.
4. Spec/QA/packet-matrix name this beside the owned host-only MYSHOP open/close seams; guest browse/buy, peer around-broadcast, stock removal, and cube busy rejects stay deferred.

## Explicit non-goals

- no guest browse / buy / sell mutation
- no peer around-broadcast of `SHOP_SIGN`
- no remembering listed stock cells for selective locks (whole-host `CanHandleItem` style)
- no cube busy rejects
- no authored `myshop_reject_message` for `anti_myshop`/`anti_give` open stock rejects (still silent)
- no claim that private-shop stock is removed from inventory

## Proof shape

1. Runtime/session: accepted MYSHOP open → each listed mutation path emits no frames and leaves inventory/equipment/quickslots/gold/ground/safebox/refine/exchange/persisted snapshot unchanged; shop stays open.
2. Negative control: `/close_myshop` then the same mutation path succeeds again under its already-owned contract.
3. Docs/spec/QA name the lock; guest browse/buy stay deferred.

## Status

Implemented on `lane/items`: open private shop fails closed for host item use/move/drop/pickup/give/safebox/refine mutations with no frames and no inventory/gold mutation; shop stays open until empty-sign close. Guest browse/buy, peer `SHOP_SIGN` around-broadcast, and cube busy rejects stay deferred. Host open reject-chat for armor / cash-item / locked / gold-overflow is owned separately (`docs/plans/2026-08-26-myshop-open-reject-chat-hardening.md`).
