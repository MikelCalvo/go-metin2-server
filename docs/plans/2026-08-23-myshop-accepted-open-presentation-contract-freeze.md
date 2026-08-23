# Accepted private-shop open presentation contract freeze — 2026-08-23

## Why this exists

`CG::MYSHOP` codec + deny-no-response GAME dispatch and `GC::SHOP_SIGN` encode/decode are now owned, but the repository still has no accepted host-only private-shop open path. Without a docs freeze, the first RED would invent stock/sign validation, busy-shell policy, and whether `GC::SHOP_SIGN` is emitted on success.

This freeze is **host-only open presentation**. It does not open guest browse/buy, partner player-shop/cube exchange busy rejects, bag-item consumption, polymorph, banword filtering, or mall.

## Oracle summary (behavior reference only)

- `CG::MYSHOP` ingress eventually calls `OpenMyShop(sign, tables, count)`
- Open fails closed for empty `count`, empty sign, gold overflow vs shop-listed prices, duplicate stock positions, missing/equipped/locked stock, and `anti_give|anti_myshop` stock
- Busy exchange / safebox / merchant / cube shells reject open with info-chat and leave the shop closed
- Successful open `PacketAround`s `GC::SHOP_SIGN` with host VID + non-empty sign, then creates the PC shop
- Close / clear reuses `GC::SHOP_SIGN` with empty sign for the same VID
- Guest browse/buy and exchange START partner-open-shop busy rejects remain later presentation seams

## Contract to freeze (before RED)

Own a first accepted host-only private-shop open path on top of the already-owned codec + deny-no-response hook:

1. Scope: selected character already in `GAME`, above the zero-HP floor, with no active same-socket exchange / merchant / safebox / refine busy presentation.
2. Ingress: valid `CG::MYSHOP` decoded into `HandleMyShop` may now return `Accepted: true` when the runtime open gate succeeds.
3. Sign / count gates:
   - `count == 0` fails closed (no frames / no open flag / no `SHOP_SIGN`)
   - empty / all-NUL sign fails closed the same way
   - `count > ShopHostItemMax` remains a codec reject
4. Stock gates (all fail closed, no open flag, no `SHOP_SIGN`):
   - every packed position must address a live carried inventory cell owned by the selected character
   - duplicate packed positions fail closed
   - equipped / locked / zero-count / missing cells fail closed
   - resolved template must not author `anti_give` or `anti_myshop`
   - listed `price` must be `> 0` and the host's current gold plus the sum of listed prices must not overflow the owned gold carrier (`>= 1<<31`)
5. Busy-shell gate: when exchange / merchant / safebox / refine is already open, reject with one self-only `CHAT_TYPE_INFO` using the same requester busy wording already owned by exchange START for those shells (`You cannot trade while another window is open.` style already frozen for merchant/safebox/refine), and do not emit `SHOP_SIGN`.
6. Success presentation (host-only):
   - remember a same-socket private-shop open/busy flag
   - emit one `GC::SHOP_SIGN` whose `vid` is the host's live shared-world entity id and whose `sign` is the decoded non-empty shop sign
   - do **not** invent guest browse frames, `GC::SHOP START` for the host, inventory removal, bag-item consumption, polymorph, or horse/mount teardown in this first slice
   - do **not** mutate carried inventory/gold on open; stock remains carried until a later sell/buy slice owns transfer
7. Close companion stays deferred as its own tiny seam unless required for lifecycle hygiene proofs in the same RED/GREEN; if included, empty-sign `GC::SHOP_SIGN` clear + open-flag clear must reuse the owned codec and must not invent guest teardown beyond the host clear.
8. Spec/QA/packet-matrix name this as the first accepted host-only open presentation; guest browse/buy, bag/pricelist/polymorph, and partner player-shop/cube exchange busy rejects stay deferred.

## Locale / wording note

Busy-shell reject text must reuse the already-owned English bootstrap strings for merchant/safebox/refine busy windows rather than inventing a new private-shop-only string in this freeze. Stock/sign fail-closed rejects may stay silent/no-frame for the first slice unless an already-owned template reject-message field is explicitly wired later.

## Explicit non-goals

- no guest browse / `AddGuest` / buy / sell mutation
- no partner-side open player-shop exchange START/ACCEPT busy rejects yet
- no cube busy rejects
- no shop-bag item (`50200` / `71049`) consumption or DB pricelist
- no armor-unequip / quest-running / banword / polymorph / mount teardown requirements in this first freeze
- no claim that deny-no-response default remains the production handler after accepted open lands — default stays fail-closed until runtime wiring opts in
- no mall

## Proof shape for the implementation slice

1. Runtime/session: valid non-empty sign + one valid carried stock row opens the busy flag and emits exactly one `GC::SHOP_SIGN` with host VID + sign; inventory/gold unchanged.
2. Negatives: empty sign / zero count / duplicate pos / missing cell / `anti_myshop` / `anti_give` / busy exchange|merchant|safebox|refine all stay fail-closed with no `SHOP_SIGN` and no open flag.
3. Docs already frozen here; guest browse/buy and partner exchange busy rejects remain untouched.

## Status

Docs/spec contract freeze on `lane/items`. Implementation RED/GREEN for the host-only accepted open presentation follows as the next cohesive slice; guest browse/buy and partner player-shop/cube exchange busy rejects stay deferred.
