# Host-only MYSHOP empty-sign close companion contract freeze — 2026-08-23

## Why this exists

Host-only accepted `CG::MYSHOP` open now remembers a same-socket busy flag and emits one live `GC::SHOP_SIGN` with a non-empty sign, but teardown still clears that flag silently on session end. Without a docs freeze, the first RED would invent close ingress, whether empty-sign emission is self-only or around, and how lifecycle paths order relative to merchant/safebox/exchange teardown.

This freeze is **host-only empty-sign clear/close presentation**. It does not open guest browse/buy, partner player-shop/cube exchange busy rejects, bag-item refund, or polymorph restore.

## Oracle summary (behavior reference only)

- `CloseMyShop()` clears the remembered sign, destroys the PC shop, and `PacketAround`s `GC::SHOP_SIGN` with host VID + empty (`szSign[0] = '\0'`) sign
- Close is invoked from disconnect / death / command paths that tear the private shop down
- Polymorph restore after close exists in the oracle but stays out of this bootstrap freeze

## Contract to freeze (before RED)

Own a first host-only empty-sign clear companion on top of the already-owned open presentation:

1. Scope: selected character already in `GAME` with `hasActiveMyShopOpen == true`.
2. Success presentation (host-only):
   - clear the same-socket private-shop open/busy flag
   - emit exactly one owned `GC::SHOP_SIGN` whose `vid` is the host's live shared-world entity id and whose `sign` is empty
   - do **not** invent guest browse teardown, bag-item refund, inventory/gold mutation, or polymorph restore in this first close slice
   - keep emission self-only for this bootstrap; peer around-broadcast of empty sign stays deferred beside guest browse
3. Close ingress for the first slice (lifecycle companions, matching the owned safebox pattern):
   - practice-mob retaliation floor that already tears merchant/safebox/exchange shells
   - exact-position transfer / warp rebootstrap that already tears those shells
   - same-socket `/phase_select` / `/quit` / `/logout` teardown
4. Ordering beside already-owned busy-shell teardown:
   - when merchant and/or exchange shells also close on the same path, keep existing `GC::SHOP END` before exchange `END` ordering
   - emit empty-sign `GC::SHOP_SIGN` after merchant `SHOP END` (when present) and before exchange `END` when those shells close together, or alone when only the private-shop flag was open
   - safebox `CloseSafebox` command-chat companions stay on their already-owned ordering; this freeze does not reorder safebox
5. Idempotency: when the private-shop flag is already clear, lifecycle paths emit no empty-sign frame for MYSHOP.
6. Optional lab slash `/close_myshop` may be included in the same GREEN if needed for focused proofs; it must reuse the same empty-sign + flag-clear helper and stay silent when already closed. No new client packet family is invented for close in this freeze.
7. Spec/QA/packet-matrix name this as the first host-only empty-sign clear companion; guest browse/buy and partner player-shop/cube exchange busy rejects stay deferred.

## Explicit non-goals

- no guest browse / buy / sell mutation
- no peer around-broadcast of empty `SHOP_SIGN` yet (self-only bootstrap)
- no partner-side open player-shop exchange START/ACCEPT busy rejects yet
- no cube busy rejects
- no shop-bag item refund / pricelist / polymorph / mount restore
- no claim that open stock is removed from inventory (open still leaves stock carried)

## Proof shape for the implementation slice

1. Runtime/session: accepted open → `/phase_select` (or quit/logout) emits one empty-sign `GC::SHOP_SIGN` with host VID and clears the busy flag; inventory/gold unchanged.
2. Negatives: already-closed lifecycle path emits no MYSHOP empty-sign frame; accepted open without lifecycle still keeps the non-empty sign path unchanged.
3. Optional: floor / transfer paths prepend the same empty-sign companion when the flag was open.
4. Docs frozen here; guest browse/buy and partner exchange busy rejects remain untouched.

## Status

Docs/spec contract freeze on `lane/items`. Implementation RED/GREEN for the host-only empty-sign clear companion follows as the next cohesive slice; guest browse/buy and partner player-shop/cube exchange busy rejects stay deferred.
