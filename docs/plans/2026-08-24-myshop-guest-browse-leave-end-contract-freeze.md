# MYSHOP guest browse leave / SHOP END contract freeze — 2026-08-24

## Why this exists

Guest browse open presentation is now owned: a visible peer can `ON_CLICK` an already-open MYSHOP host and receive one guest-only `GC::SHOP START` stock table, with a same-socket guest-browse-open flag remembered for a later leave seam. Without a leave/`END` freeze, the next RED would invent whether guest close reuses merchant `CG::SHOP END`, whether host empty-sign close should force guest teardown, and whether lifecycle teardown must emit guest `SHOP END`.

The external oracle's `CShop::RemoveGuest` clears the guest shop pointer and emits one bare `GC::SHOP` / `END` to that guest. Guest leave stays presentation-only until buy/sell exists.

This freeze is **guest browse leave / END only**. It does not invent buy/sell mutation or cube busy rejects.

## Oracle summary (behavior reference only)

- Guest leave / RemoveGuest emits one bare `GC::SHOP END` to the guest and clears that guest's shop association
- Host shop close / destroy also removes guests with the same END path
- No inventory/gold mutation on leave
- Buy / sell remain separate seams

## Contract to freeze (before RED)

Own the first guest browse leave path on top of already-owned guest browse open:

1. Scope: guest already has `activeGuestMyShopHostVID != 0` from a successful browse open against a host VID.
2. Explicit guest close ingress: valid `CG::SHOP END` (`HandleShopClose`) may clear guest browse when that flag is set, even when no merchant window is open:
   - emit exactly one self-only `GC::SHOP END`
   - clear `activeGuestMyShopHostVID`
   - do not mutate inventory/gold and do not invent host `SHOP_SIGN` / host close
3. Host empty-sign close / Leave / reclaim while a guest is browsing that host:
   - guest receives one queued self-only `GC::SHOP END`
   - guest clears `activeGuestMyShopHostVID`
   - host empty-sign / busy-bit / remembered stock clear stays the already-owned host path
4. Guest lifecycle teardown (`/phase_select` / `/quit` / `/logout`, practice-mob floor, transfer/warp, session Leave):
   - if guest browse is open, emit/prepend one self-only `GC::SHOP END` before already-owned merchant/myshop/safebox/exchange teardown ordering for that guest socket
   - clear the guest browse flag
5. Already-closed / no browse flag:
   - `CG::SHOP END` with no merchant and no guest browse stays silent/no-frame (existing merchant-only close behavior)
6. Spec/QA/packet-matrix name guest browse leave beside owned browse open; buy/sell and cube busy rejects stay deferred.

## Explicit non-goals

- no buy / sell / gold / inventory mutation
- no host stock removal on guest leave
- no distance-on-leave auto-close beyond already-owned host empty-sign / lifecycle paths
- no cube busy rejects beyond naming them deferred
- no claim that private-shop trading is playable end-to-end once guest leave exists

## Proof shape for the implementation slice

1. Runtime/session: guest browses open host → guest `CG::SHOP END` emits one `GC::SHOP END`, clears browse flag, inventory/gold unchanged; later second END is silent.
2. Host `/close_myshop` while guest is browsing queues one guest `GC::SHOP END` beside the owned host empty-sign path.
3. Docs/spec/QA name the leave seam; buy/sell stay deferred.

## Status

Implemented on `lane/items`: guest `CG::SHOP END` while browsing emits one guest-only `GC::SHOP END` and clears shared-world browse association; host empty-sign close / Leave / reclaim queues the same guest END; guest lifecycle teardown prepends one guest END when browse is open. Buy/sell and cube busy rejects stay deferred.
