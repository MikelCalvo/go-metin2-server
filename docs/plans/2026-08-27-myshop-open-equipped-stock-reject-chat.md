# MYSHOP open equipped-stock INFO reject chat — 2026-08-27

## Objective

Close the remaining oracle-shaped host-only `CG::MYSHOP` stock feedback gap
after armor / cash-item / locked / gold-overflow / shop-bag `50200`: when a
listed stock row resolves a live equipped item, emit self-only INFO chat and
keep the shop closed — without inventing silk-bag `71049`, banword,
quest-running, or authored `myshop_reject_message`.

## Why this exists

Accepted open now requires-and-consumes ordinary shop bag `50200`. Equipped
stock still collapses into the silent structural / missing-cell path. The
external `OpenMyShop` oracle emits INFO when `pkItem->IsEquipped()` before the
locked check. Manual QA therefore sees a silent fail when listing worn gear
instead of the client-visible equipped reject. Track C after the bag gate
prefers this fail-closed feedback seam over deferred display/economy work.

## Contract to freeze (before RED / GREEN)

1. **Equipped-stock gate** (during the stock walk, after
   `anti_give|anti_myshop`, before locked): when a listed stock row resolves a
   live item that is equipped — equipment-window position, or a live instance
   with `Equipped: true` — emit one self-only `CHAT_TYPE_INFO`
   `Equipped items cannot be sold in a private shop.` (first offending row
   wins), leave the shop closed, emit no bag debit / no `SHOP_SIGN`, and mutate
   nothing.
2. **Gate ordering** on the accepted decode path (unchanged earlier gates):
   death-floor / no-selected / no-shared → silent;
   empty sign / zero count → silent;
   busy shells → owned busy info-chat;
   body-armor → armor chat;
   stock structural fails (empty/missing/count-vnum/dup/zero-price/non-carried
   window without an equipped live item) stay **silent**;
   `anti_give|anti_myshop` → cash-item chat;
   **equipped** → equipped chat;
   locked → locked chat;
   gold overflow → gold chat;
   shop-bag → owned require/consume;
   success → owned bag refresh + `SHOP_SIGN`.
3. Spec/QA/packet-matrix/roadmap name this chat beside owned MYSHOP open once
   GREEN; until then this freeze is the source of truth for the next RED.
4. Do **not** invent bag-missing INFO, silk-bag `71049`, banword, quest-running,
   authored `myshop_reject_message`, or polymorph/mount teardown in this slice.

## Locale / wording note

Oracle uses Korean `LC_TEXT` for the equipped-stock reject. This bootstrap
freeze uses the project-owned English string above (same pattern as armor /
cash-item / locked / gold-overflow freezes). Do not copy oracle source strings.

## Explicit non-goals

- silk-bag `71049` consume-skip + `MYSHOP_PRICELIST` DB packets
- banword / Canada locale sign filtering
- quest-running open block
- authored `myshop_reject_message` template field
- bag-missing INFO chat (ordinary bag miss stays silent)
- polymorph / horse / mount teardown on open
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Runtime/session: list an equipped / equipment-window stock row → one
   equipped INFO chat only; no bag debit; no `SHOP_SIGN`; inventory/gold
   unchanged.
2. Negative: cash-item / locked / armor chats still win in their owned order
   before / around this gate; ordinary missing-cell structural fails stay
   silent; valid open with bag still consumes `50200` then emits `SHOP_SIGN`.

## Status

Implemented on `lane/items`: listed equipped / equipment-window stock rejects
with self-only INFO chat after cash-item and before locked; no bag debit / no
`SHOP_SIGN`; silk-bag `71049` and other deferred open gates stay out of scope.
