# MYSHOP peer SHOP_SIGN around-broadcast contract freeze — 2026-08-24

## Why this exists

Host-only accepted `CG::MYSHOP` open/close already emit self-only live/empty `GC::SHOP_SIGN`, remember a same-socket busy flag, reject exchange while open, and fail closed for host item mutations. Visible peers still never receive the sign, so manual multi-client QA cannot see an open private shop on another character and guest browse remains blocked behind missing peer presentation.

The external oracle `PacketAround`s both live and empty `GC::SHOP_SIGN` on open/close, and also delivers the remembered live sign when a peer enters view of an already-open shop. Without a docs freeze, the first RED would invent whether host self frames stay duplicated, whether view-entry rematerialization is in scope, and how empty-sign close fans out.

This freeze is **peer around-broadcast of already-owned live/empty `GC::SHOP_SIGN`**. It does not invent guest browse/buy, stock removal, bag consumption, or cube busy rejects.

## Oracle summary (behavior reference only)

- Successful `OpenMyShop` builds `TPacketGCShopSign` with host VID + non-empty sign and `PacketAround`s it
- `CloseMyShop` builds the same packet with empty sign and `PacketAround`s it
- When a descriptor enters view of a character that still has `GetMyShop()`, the viewer receives one live `GC::SHOP_SIGN` for that host VID + remembered sign
- Guest browse/buy remain separate presentation seams after the sign is visible

## Contract to freeze (before RED)

Own peer fanout on top of the already-owned host-only open/close helpers:

1. Scope: selected character already in `GAME`, joined to shared world, with a visible same-map peer session that is not at the bootstrap HP floor.
2. Accepted open (`setActiveMyShopOpen` success path):
   - keep the existing host-return live `GC::SHOP_SIGN` frame unchanged
   - additionally `EnqueueToVisibleSessions` the same live `GC::SHOP_SIGN` bytes to currently visible peer sessions
   - do not invent a second host self frame through the peer queue
3. Empty-sign close companions (`closeActiveMyShopOpenFrames` / lifecycle / lab `/close_myshop`):
   - keep the existing host-return empty-sign frame unchanged
   - additionally `EnqueueToVisibleSessions` the same empty-sign bytes to currently visible peer sessions when the flag was open
   - already-closed close stays silent for both host and peers
4. View-entry rematerialization (same slice if the shared-world join/visibility path already has a stable hook; otherwise keep as an explicit follow-on named in Status):
   - when a peer becomes newly visible to a host that still has `hasActiveMyShopOpen` / peer-visible MyShop busy bit, deliver exactly one live `GC::SHOP_SIGN` for that host VID + remembered non-empty sign to the newly visible peer
   - do not invent guest browse frames or stock tables on view entry
5. Ordering / hygiene:
   - peer fanout uses the already-owned `EnqueueToVisibleSessions` helper and skips HP-floor peers the same way other peer item/appearance frames do
   - merchant `SHOP END` / exchange `END` host ordering stays unchanged; peer sign fanout is additive beside those host frames
6. Spec/QA/packet-matrix name peer around-broadcast beside the owned host-only MYSHOP open/close seams; guest browse/buy and cube busy rejects stay deferred.

## Explicit non-goals

- no guest browse / `AddGuest` / buy / sell mutation
- no stock removal from host inventory on open
- no shop-bag consumption / pricelist / polymorph / mount teardown
- no cube busy rejects
- no authored `myshop_reject_message` for silent stock rejects
- no claim that private-shop trading is playable end-to-end once the sign is visible

## Proof shape for the implementation slice

1. Runtime/session: host + visible peer both in GAME → accepted MYSHOP open returns one live `SHOP_SIGN` to the host and queues the same live `SHOP_SIGN` on the peer; inventory/gold unchanged.
2. Close: `/close_myshop` (or lifecycle) returns one empty-sign frame to the host and queues the same empty-sign frame on the peer; busy flag clears.
3. Optional/follow-on: peer joins visibility of an already-open host and receives one live rematerialized `SHOP_SIGN` without inventing browse frames.
4. Docs/spec/QA name the around-broadcast; guest browse/buy stay deferred.

## Status

Implemented on `lane/items` for open/close peer around-broadcast of live/empty `GC::SHOP_SIGN`. View-entry rematerialization of a remembered live sign is owned separately (`docs/plans/2026-08-24-myshop-peer-shop-sign-view-entry-rematerialization-contract-freeze.md`); guest browse/buy and cube busy rejects stay deferred.
