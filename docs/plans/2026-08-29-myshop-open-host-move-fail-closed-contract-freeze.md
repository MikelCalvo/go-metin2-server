# MYSHOP open host MOVE / SyncPosition fail-closed — 2026-08-29

## Objective

Freeze the first open-private-shop host movement deny so an accepted
`CG::MYSHOP` busy flag (`hasActiveMyShopOpen`) blocks host `CG::MOVE` /
`CG::SYNC_POSITION` the same way the external oracle's `CHARACTER::CanMove()`
returns false while `GetMyShop()` is live — without inventing walk-away shop
close, movement reject chat, shopkeeper polymorph, quest-running, or GD/DB
pricelist packets.

## Why this exists

Host-only accepted private-shop open already owns:
- same-socket `hasActiveMyShopOpen` + live/empty `GC::SHOP_SIGN`
- host item-mutation fail-closed lock while open
- exchange / guest-browse busy bits
- durable FileStore `myshop_unit_prices` rematerialize for silk bag USE

Stock remains carried until sold, and the host can still walk while the shop
is open. That diverges from oracle `CanMove()` (`char.cpp`: `if (GetMyShop())
return false`) and invents an unsafe bootstrap where the open host relocates
under a still-visible peer `SHOP_SIGN`. Safebox already owns walk-away
auto-close; MYSHOP must **not** copy that — oracle keeps the shop open and
simply denies movement.

This freeze is **host movement fail-closed while MYSHOP is open**. It does not
invent guest distance buy gates beyond the already-owned buy path, polymorph,
quest-running, bag-missing INFO, or GD/DB `MYSHOP_PRICELIST_*`.

## Contract to freeze (before RED)

While the same-socket private-shop open/busy flag is set:

1. Fail closed with **no frames** / no live or persisted position mutation /
   no shared-world position update / no transfer-trigger evaluation for:
   - packet `CG::MOVE` (`HandleMove`)
   - packet `CG::SYNC_POSITION` elements that target the selected character VID
     (`HandleSyncPosition`)
2. Do **not** invent a new info-chat string; silent no-frame deny matches the
   owned open-MYSHOP host item-mutation lock style and oracle `CanMove`
   (boolean gate, no chat).
3. Do **not** auto-close the private shop on denied movement. Shop stays open
   with the same live `SHOP_SIGN` / peer-visible busy bit until empty-sign
   close / lifecycle teardown / floor / transfer already owned elsewhere.
4. Guest browse distance / buy fail-closed paths stay unchanged.
5. Spec/QA/roadmap name this beside the owned host MYSHOP open/item-lock
   seams; walk-away close, polymorph, quest-running, and GD/DB pricelist stay
   deferred.

## Locale / wording note

No new English INFO string. Silent deny only.

## Explicit non-goals

- walk-away / distance auto-close of an open private shop
- shopkeeper `SetPolymorph(30000)` / mount / horse teardown
- quest-running (`PC::IsRunning`) open block
- bag-missing INFO (oracle ordinary-bag miss stays silent)
- GD/DB `MYSHOP_PRICELIST_*` packets / SQL
- guest-buy tax / empire multipliers
- refine catalysts
- changing the owned host item-mutation lock or empty-sign close path

## Proof shape (later RED/GREEN — not this freeze)

1. Session: accepted silk or bag `CG::MYSHOP` open → host `MOVE` / self
   `SYNC_POSITION` emit no frames and leave live/persisted position + shop
   open flag + `SHOP_SIGN` unchanged.
2. Negative control: `/close_myshop` (empty sign) then the same move path
   succeeds under the already-owned movement contract.
3. Docs/spec/QA name the deny; walk-away close / polymorph stay deferred.

## Status

Contract freeze for lane/items. RED/GREEN deferred until this freeze lands.
