# MYSHOP peer SHOP_SIGN view-entry rematerialization contract freeze — 2026-08-24

## Why this exists

Open/close peer around-broadcast of live/empty `GC::SHOP_SIGN` is already owned. A peer who only becomes visible *after* the host already opened still never receives the remembered live sign, so late joiners / walk-into-view clients cannot see the open private shop until a later open/close event.

The external oracle rematerializes the remembered live sign when a descriptor enters view of a character that still has `GetMyShop()`. Without a docs freeze, the first RED would invent whether rematerialization rides Join visibility bootstrap, relocate/transfer visibility diffs, remembered sign storage on the shared-world busy bit, or only same-map EnterGame peers.

This freeze is **view-entry rematerialization of an already-open host's remembered live `GC::SHOP_SIGN`**. It does not invent guest browse/buy, stock mutation, or cube busy rejects.

## Oracle summary (behavior reference only)

- When a descriptor enters view of a character that still has `GetMyShop()`, the viewer receives one live `GC::SHOP_SIGN` for that host VID + remembered sign
- Open/close `PacketAround` fanout remains separate and already owned in this repo
- Guest browse/buy remain separate presentation seams after the sign is visible

## Contract to freeze (before RED)

1. Scope: host already has `hasActiveMyShopOpen` / peer-visible MyShop busy bit with a remembered non-empty sign; a peer becomes newly visible to that host (Join of the peer into the host's visibility, or host relocate/transfer that adds the peer).
2. Delivery:
   - deliver exactly one live `GC::SHOP_SIGN` (`host VID` + remembered non-empty sign) to the newly visible peer
   - do not invent a second character-add burst or guest browse/stock frames
   - do not re-emit empty-sign clear on view entry
3. Storage:
   - remember the accepted non-empty sign beside the existing same-socket open flag / shared-world MyShop busy bit for the life of the open presentation
   - clear remembered sign on empty-sign close / Leave / reclaim the same way the busy bit clears
4. Ordering / hygiene:
   - rematerialized sign rides after ordinary peer visibility bootstrap frames for that host (character add/info/update) when those frames are already being delivered
   - skip bootstrap HP-floor peers the same way other peer item/appearance frames do
5. Spec/QA/packet-matrix name view-entry rematerialization beside the owned open/close around-broadcast; guest browse/buy and cube busy rejects stay deferred.

## Explicit non-goals

- no guest browse / `AddGuest` / buy / sell mutation
- no stock removal from host inventory on open
- no shop-bag consumption / pricelist / polymorph / mount teardown
- no cube busy rejects
- no claim that private-shop trading is playable end-to-end once late joiners see the sign

## Proof shape for the implementation slice

1. Runtime/session: host opens MYSHOP while peer is out of view (or peer joins after open) → newly visible peer receives exactly one live rematerialized `SHOP_SIGN` with host VID + remembered sign; inventory/gold unchanged.
2. Negative: host closes before the peer becomes visible → peer receives no live rematerialized sign for that host.
3. Docs/spec/QA name the rematerialization; guest browse/buy stay deferred.

## Status

Docs-first freeze on `lane/items`. Implementation RED intentionally deferred until this contract is committed green on `main`-rebased lane history.
