# MYSHOP guest SELL while browsing fail-closed — 2026-08-25

## Objective

Prove and name the already-owned oracle PC-shop sell reject so guest
`CG::SHOP SELL` / `SELL2` while browsing an open MYSHOP stay fail-closed with
no frames and no inventory/gold/host mutation.

## Why this exists

Guest private-shop buy is owned, and the buy contract freeze already said guest
sell-into-PC-shop stays fail-closed / deferred. Runtime already routes
`HandleShopSell` / `HandleShopSell2` only through `executeActiveMerchantSell`,
which requires an open NPC merchant window (`hasActiveMerchantBuy`), so browse-
only private-shop guests already get silent `Accepted: false`. Track C's next
named item after cube busy rejects is this explicit proof + docs ownership so
QA/spec stop calling the seam deferred.

## Contract owned by this slice

1. While `activeGuestMyShopHostVID != 0` and no same-socket NPC merchant window
   is open, guest `CG::SHOP SELL` and `CG::SHOP SELL2` fail closed:
   - no frames (no shop error companion, no inventory/gold refresh, no host
     queued frames)
   - no inventory / gold / quickslot / host stock / persistence mutation
   - guest browse flag and host MYSHOP stay open
2. Oracle behavior reference: `CShopManager::Sell` returns immediately when
   `ch->GetShop()->IsPCShop()` is true.
3. Spec/QA/packet-matrix/roadmap name this as owned fail-closed beside guest
   buy; do not invent guest sell-into-PC-shop mutation, tax/empire multipliers,
   or shop-bag consumption.
4. Close the stale Track C Next that still pointed at guest browse / safebox /
   refine cube busy rejects (those are already GREEN).

## Explicit non-goals

- no accepted guest sell into a private shop
- no tax / empire price multipliers / shop-bag consumption
- no `cube add` / `delete` / `list` / `make` / recipe mutation
- no mall / TMP4 CG `SAFEBOX_MONEY`

## Proof shape

1. Runtime/session: host opens MYSHOP → guest browses → guest `SHOP SELL` and
   `SHOP SELL2` of a carried cell emit no frames, queue no host frames, and leave
   both accounts unchanged.
2. Docs/spec/QA rename the seam from deferred to owned fail-closed.

## Status

Implemented on `lane/items`:
`TestGameRuntimeMyShopGuestSellWhileBrowsingFailsClosedWithoutMutation` proves
guest `SELL` / `SELL2` while browsing stay silent/no-mutation. Recipe
make/add/list, tax/empire multipliers, and shop-bag consumption stay deferred.
