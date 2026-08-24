# MYSHOP guest browse open presentation contract freeze — 2026-08-24

## Why this exists

Host-only accepted `CG::MYSHOP` open/close, peer live/empty `GC::SHOP_SIGN` around-broadcast, and view-entry rematerialization are already owned. Visible peers can see an open private-shop sign, but clicking that peer still hits the fail-closed `ON_CLICK` ingress guard and never opens a guest `GC::SHOP START` stock table, so manual multi-client QA cannot browse another player's private shop.

The external oracle opens guest browse through `CG::ON_CLICK` on a PC that still has `GetMyShop()`, then `AddGuest` emits one ordinary `GC::SHOP START` (`owner_vid` + fixed 40-slot table). Without a docs freeze, the first RED would invent whether browse reuses merchant `INTERACT`, whether stock must be removed from host inventory first, whether sockets/attrs are snapshotted at open or read live, and whether a distance gate applies on open.

This freeze is **guest browse open presentation only**: visible already-open host → peer `ON_CLICK` → busy gates → one `GC::SHOP START` stock table. It does not invent buy/sell mutation, guest `SHOP END` leave productization beyond naming it deferred, or cube busy rejects.

## Oracle summary (behavior reference only)

- Guest does **not** send `CG::MYSHOP` to browse; client sends `CG::ON_CLICK` with the host VID
- Server `OnClick` routes a PC with live MyShop into `AddGuest` (after ordinary silent/busy rejects)
- Success emits one `GC::SHOP` / `START` with `owner_vid` + fixed `items[40]` (`vnum`, `price`, `count`, `display_pos`, sockets, attributes)
- Table is zero-filled then filled by shop display index; empty / missing live item slots stay empty
- PC click passes no empire price multiplier; browse does not re-emit `SHOP_SIGN`
- Distance gate `SHOP_MAX_DISTANCE` is NPC shopping only; private-shop open browse has no distance reject
- Buy / sell / gold / inventory mutation and guest leave/`END` remain separate seams

## Contract to freeze (before RED)

Own the first guest browse open path on top of already-owned MYSHOP open + peer sign visibility + `ON_CLICK` codec/dispatch:

1. Scope: guest selected character already in `GAME`, above the zero-HP floor, with a visible same-map peer host that currently has `hasActiveMyShopOpen` / shared-world MyShop busy bit + remembered non-empty sign.
2. Ingress: `CG::ON_CLICK` (`0x0A02`) targeting that host VID may become the guest-browse open path when the host still has an open private shop. Non-MyShop / unsupported click targets keep the existing fail-closed no-frame guard.
3. Remembered stock (host open prerequisite):
   - accepted host `CG::MYSHOP` must remember the validated stock listing beside the open flag / shared-world busy bit / sign: per listed row `display_pos`, `vnum`, `count`, `price`, and source carried cell
   - empty-sign close / Leave / reclaim clears that remembered listing with the busy bit and sign
   - open still does **not** remove carried stock or mutate gold; host item-mutation lock already owned while open stays in force
4. Busy / reject gates (fail closed, no `GC::SHOP START`):
   - guest silent/no-frame when guest is dead / at bootstrap HP floor, or when guest currently has own open MYSHOP
   - guest self-only busy info-chat using the already-owned requester busy string (`You cannot trade while another trade window is open.`) when guest has open merchant / safebox / refine / exchange (and, if already browsing another private shop in a later leave seam, keep that reject named but do not invent cube wording here)
   - host-side reject with the already-owned partner busy string (`That player cannot trade right now.`) when the host currently has open exchange / safebox / refine (host open MYSHOP is required for success and is not itself a reject)
   - unknown / non-open-MyShop peer VID stays the existing silent `ON_CLICK` no-op
5. Success presentation (guest-only):
   - emit exactly one `GC::SHOP START` using the already-owned `EncodeServerStart` shape (`OwnerVID` = host VID + fixed `[ShopHostItemMax]` table)
   - fill slots by remembered `display_pos`; each occupied slot carries remembered `vnum` / `count` / `price` / `display_pos`
   - project sockets/attributes from the host's current live matching carried cell when that cell still matches remembered `vnum`+`count`+source slot; otherwise leave that display slot empty rather than inventing buy-time repair
   - do **not** re-emit `SHOP_SIGN`, do **not** mutate guest or host inventory/gold, and do **not** invent `START_EX` / NPC tab shops
   - remember a same-socket guest-browse-open presentation flag keyed to the host VID so a later leave/`END` slice can clear it; this freeze does not yet own guest `SHOP END` / RemoveGuest productization
6. Ordering / hygiene:
   - guest browse open does not invent a distance gate
   - host open / peer around-broadcast / view-entry rematerialization stay unchanged
   - merchant `INTERACT` open path stays NPC/static-actor only
7. Spec/QA/packet-matrix name guest browse open beside owned MYSHOP sign seams; buy/sell, guest leave/`END`, stock removal on open, and cube busy rejects stay deferred.

## Explicit non-goals

- no buy / sell / gold / inventory mutation
- no guest `GC::SHOP END` / RemoveGuest leave productization yet (only remember enough state that a follow-on can clear it)
- no distance-on-open reject for PC private-shop browse
- no NPC `StartShopping` / `START_EX` / tab shops
- no shop-bag consumption / pricelist / polymorph / mount teardown
- no cube busy rejects beyond naming them deferred
- no claim that private-shop trading is playable end-to-end once browse START exists

## Proof shape for the implementation slice

1. Runtime/session: host opens MYSHOP with one listed carried row → visible peer `ON_CLICK` host VID → peer receives exactly one `GC::SHOP START` with host VID and that display slot filled; inventory/gold unchanged on both sides.
2. Negatives: guest open merchant/safebox/refine/exchange → busy info-chat and no START; host without open MYSHOP / closed before click → silent no-frame; guest own open MYSHOP → silent no-frame.
3. Docs/spec/QA name the guest browse open seam; buy/sell and guest leave/`END` stay deferred.

## Status

Implemented on `lane/items`: guest `CG::ON_CLICK` against a visible already-open MYSHOP host emits one guest-only `GC::SHOP START` stock table from the remembered listing + live matching carried cell sockets/attrs; busy guest/host shells reuse owned exchange busy info-chat strings; guest own open MYSHOP / closed host stay silent. Buy/sell, guest leave/`END`, and cube busy rejects stay deferred.
