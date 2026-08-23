# Exchange MyShop Busy-Window Reject Chat — 2026-08-24

## Objective

Once host-only private-shop open/close presentation exists, freeze the first partner-visible exchange busy-window reject so an open `MYSHOP` blocks `EXCHANGE START` / `ACCEPT` / commit the same way merchant / safebox / refine already do.

## Why this exists

Host-only accepted `CG::MYSHOP` open now remembers a same-socket busy flag and clears it with empty-sign `GC::SHOP_SIGN`, but that flag is still local-only. Exchange START/ACCEPT/commit partner busy checks therefore cannot yet see a peer private shop, and the roadmap still defers partner player-shop busy rejects.

This freeze is **exchange busy-window reject chat for open private shop**. It does not invent guest browse/buy, peer around-broadcast of `SHOP_SIGN`, or cube busy rejects.

## Contract to freeze (before RED)

Own open-private-shop busy rejects beside the already-owned merchant/safebox/refine busy gate:

1. Requester-side open private shop (`hasActiveMyShopOpen`) rejects:
   - `EXCHANGE START`
   - `EXCHANGE ACCEPT` (first or second)
   - commit-time busy drift after a second-accept finalize plan
   with one self-only `CHAT_TYPE_INFO` using the already-owned requester busy string (`You cannot trade while another trade window is open.`).
2. Partner-side open private shop rejects the same seams with one self-only `CHAT_TYPE_INFO` using the already-owned partner busy string (`That player cannot trade right now.`).
3. When both sides are busy, requester busy text wins (local-first), matching merchant/safebox/refine ordering.
4. No exchange frames / no pairing mutation on `START`; no accept marker / no finalize frames / no inventory-gold mutation on `ACCEPT`/commit; the private-shop flag and empty-sign path remain unchanged by the reject.
5. Shared-world visibility: publish a peer-visible open-private-shop busy bit (same shape as `SetMerchantWindowOpen` / `SetRefineWindowOpen`) so partner START/ACCEPT/commit can observe it without inventing guest browse.
6. Spec/QA name private-shop beside merchant/safebox/refine in the exchange busy matrix; cube busy rejects stay deferred.

## Explicit non-goals

- no guest browse / buy / sell mutation
- no peer around-broadcast of `SHOP_SIGN` beyond the already-owned self-only open/close companion
- no cube busy rejects
- no authored/template-backed overrides for busy-window strings
- no claim that private-shop stock is removed from inventory

## Proof shape for the implementation slice

1. Runtime/session: open MYSHOP on requester → `EXCHANGE START` emits requester busy chat and no pairing.
2. Runtime/session: open MYSHOP on partner → requester `EXCHANGE START` emits partner busy chat and no pairing; partner shop stays open.
3. ACCEPT / second-accept / commit-time busy drift mirror the same strings with no mutation.
4. Docs frozen here; cube busy rejects remain untouched.

## Status

Docs/spec contract freeze on `lane/items`. Implementation RED/GREEN for open-private-shop exchange busy rejects follows as the next cohesive slice; guest browse/buy and cube busy rejects stay deferred.
