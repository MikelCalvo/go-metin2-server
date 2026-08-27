# MYSHOP silk-bag (`71049`) consume-skip gate — 2026-08-27

## Objective

Close the remaining host-only `CG::MYSHOP` bag-branch gap after ordinary shop
bag `50200` require-and-consume and equipped-stock INFO reject: when the host
carries at least one unlocked ordinary silk bag (`vnum = 71049`) that is not
also listed in this open's stock table, accepted open must **skip** the `50200`
consume and still emit the owned bag-less success path (`SHOP_SIGN` only) —
without inventing `MYSHOP_PRICELIST` DB packets, banword, quest-running, or
polymorph/mount teardown in this slice.

## Why this exists

Accepted open currently requires-and-consumes `50200` after armor / cash /
equipped / locked / gold gates. The external `OpenMyShop` oracle prefers
`CountSpecifyItem(71049)` first: silk bag is **not** removed, ordinary `50200`
is skipped, and only then does the oracle open. Manual QA with a silk bag but
no ordinary bag therefore still sees a silent fail. Track C prefers this
consume-skip economy seam before inventing DB pricelist / banword /
quest-running display work.

## Contract to freeze (before RED / GREEN)

1. **Gate ordering** on the accepted decode path (unchanged earlier gates):
   death-floor / no-selected / no-shared → silent;
   empty sign / zero count → silent;
   busy shells → owned busy info-chat;
   body-armor → armor chat;
   stock structural fails → silent;
   `anti_give|anti_myshop` → cash-item chat;
   equipped → equipped chat;
   locked → locked chat;
   gold overflow → gold chat;
   **then bag branch**:
   - if host has ≥1 carried unlocked unequipped `vnum = 71049` whose cell is
     **not** listed in this open's stock table → **silk path** (no `50200`
     consume, no inventory/quickslot mutation from the bag branch);
   - else fall through to the already-owned ordinary `50200` require/consume;
   - else silent fail-closed (no frames / no open).
2. **Silk success**: set the open/busy flag and emit the owned live
   `GC::SHOP_SIGN` (+ peer around-broadcast as already owned) with **no** bag
   `ITEM_UPDATE` / `ITEM_DEL` / `QUICKSLOT_DEL` frames and with inventory /
   quickslots / gold unchanged by the bag branch.
3. **Listed-only / locked-only silk bags do not count** for the silk path
   (same exclusion rule already owned for ordinary `50200`). A listed silk bag
   does not unlock consume-skip by itself.
4. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open once GREEN;
   until then this freeze is the source of truth for the next RED.
5. Do **not** invent `MYSHOP_PRICELIST` / DB cache packets, bag-missing INFO,
   banword, quest-running, authored `myshop_reject_message`, or
   polymorph/horse/mount teardown in this slice.

## Locale / wording note

No new INFO chat is invented for the silk path. Success is bag-less
`SHOP_SIGN`; ordinary-bag miss without silk stays the owned silent fail.

## Explicit non-goals

- `MYSHOP_PRICELIST` / `GD::MYSHOP_PRICELIST_UPDATE` / DB price-list rematerialize
- banword / Canada locale sign filtering
- quest-running open block
- authored `myshop_reject_message` template field
- bag-missing INFO chat
- polymorph / horse / mount teardown on open
- removing listed carried stock into shop ownership on open
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Runtime/session: carried unlisted `71049` and no `50200` → open emits exactly
   one `SHOP_SIGN` (no bag refresh); inventory / quickslots / gold unchanged;
   account snapshot unchanged by the bag branch.
2. Runtime/session: carried unlisted `71049` **and** carried `50200` → silk path
   wins (no `50200` debit; still one `SHOP_SIGN` only).
3. Negative: listed-only / locked-only `71049` does not unlock silk path; fall
   through to ordinary `50200` require/consume or silent miss.
4. Negative: armor / cash / equipped / locked / gold chats still win before the
   bag branch; ordinary `50200`-only open still consumes as owned.

## Status

Docs/spec freeze only on `lane/items`. Runtime RED/GREEN intentionally deferred
to the next implementation slice.
