# MYSHOP open reject-chat hardening — 2026-08-26

## Objective

Upgrade the already-owned host-only `CG::MYSHOP` open path so oracle-shaped
client-visible reject feedback replaces silent no-frame fails for body-armor
gate, `anti_give|anti_myshop` stock, locked stock, and gold-overflow — without
inventing shop-bag consumption, banword filtering, quest-running blocks, or
authored `myshop_reject_message`.

## Why this exists

Host-only open, empty-sign close, peer sign fanout, guest browse/buy, and host
mutation lock are owned. Stock/`anti_myshop`/`anti_give`/locked/gold-overflow
rejects still emit **no frames**, and worn body armor (`EquipmentSlotBody`) is
not gated at all. The external behavior oracle's `OpenMyShop` emits INFO chat
for those cases and rejects open when `PART_MAIN > 2`. Manual QA therefore
sees silent fails or can open while armored, diverging from client-visible
oracle feedback. Roadmap Track C after cube `list`/`cancel` prefers this
economy hardening gap over inventing OR-materials / binary cube headers.

## Contract to freeze (before / with GREEN)

1. **Body-armor gate** (after busy-shell reject, before stock walk): when the
   selected character's live equipment already occupies `EquipmentSlotBody`,
   emit one self-only `CHAT_TYPE_INFO`
   `You must unequip your armor to open a private shop.`, leave the shop
   closed, emit no `SHOP_SIGN`, and mutate nothing.
2. **`anti_give|anti_myshop` stock**: when a listed carried cell resolves a
   template with `AntiGive` or `AntiMyShop`, emit one self-only `CHAT_TYPE_INFO`
   `Cash items cannot be sold in a private shop.` (first offending row wins),
   no `SHOP_SIGN`, no open flag, no mutation. Do **not** add
   `myshop_reject_message` in this slice; fixed English only.
3. **Locked stock**: when a listed live carried cell is `Locked`, emit one
   self-only `CHAT_TYPE_INFO`
   `Items currently in use cannot be sold in a private shop.`, same no-open /
   no-mutation contract.
4. **Gold-overflow**: when host live gold plus the running sum of listed prices
   would exceed `math.MaxInt32`, emit one self-only `CHAT_TYPE_INFO`
   `You cannot open a private shop because it would exceed 2 Billion Yang.`,
   same no-open / no-mutation contract.
5. **Gate ordering** on accepted decode path: death-floor / no-selected /
   no-shared → silent; empty sign / zero count → silent; busy shells → owned
   busy info-chat; body-armor → armor chat; then stock walk structural
   fails (window/cell/display/dup/zero-price/count/vnum mismatch/missing) stay
   **silent**; anti → cash-item chat; locked → locked chat; gold overflow →
   gold chat; success → owned `SHOP_SIGN`.
6. Spec/QA/packet-matrix/roadmap name these chats beside owned MYSHOP open;
   shop-bag / banword / quest-running / polymorph / mount / authored
   `myshop_reject_message` stay deferred.

## Locale / wording note

Oracle uses Korean `LC_TEXT` for armor / cash-item / locked / gold-overflow.
This bootstrap freeze uses project-owned English strings above (same pattern as
safebox / exchange busy chat freezes). Do not copy oracle source strings.

## Explicit non-goals

- shop-bag item (`50200` / `71049`) consume / require-bag gate
- banword / Canada locale sign filtering
- quest-running open block
- authored `myshop_reject_message` template field
- equipped-stock INFO chat beyond the existing silent missing-cell path
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Runtime/session: armor equipped → MYSHOP emits armor INFO only; no open.
2. Runtime/session: `anti_myshop` / `anti_give` stock → cash-item INFO only.
3. Runtime/session: locked stock → locked INFO only.
4. Runtime/session: gold overflow → gold INFO only.
5. Negative: empty sign / missing cell stay silent; valid open still emits one
   `SHOP_SIGN`.

## Status

Implemented on `lane/items` together with this freeze.
