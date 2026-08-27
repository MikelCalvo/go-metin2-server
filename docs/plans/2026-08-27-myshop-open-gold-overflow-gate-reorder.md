# MYSHOP open gold-overflow gate reorder — 2026-08-27

## Objective

Close the remaining host-only `CG::MYSHOP` open ordering gap after
`myshop_reject_message`: move the gold-overflow INFO reject so it fires after
armor and **before** banword / stock walk, matching the external `OpenMyShop`
price-sum gate without inventing quest-running open blocks,
`MYSHOP_PRICELIST`, Canada banword bypass, bag-missing INFO, or tax/empire
multipliers.

## Why this exists

Bootstrap currently accumulates listed prices inside the stock walk and only
emits gold-overflow after cash / equipped / locked checks pass for that row.
When a request is both over the gold carrier cap and would later hit banword or
cash-item stock, QA sees banword/cash INFO instead of the gold-overflow string
the TMP4 oracle would emit first. Track C prefers this client-visible ordering
fix over inventing DB pricelist rematerialize, quest-running, OR-materials, or
binary cube headers.

## Contract to freeze (before RED)

1. **Gold gate position** on the accepted decode path:
   death-floor / no-selected / no-shared → silent;
   empty sign / zero count → silent;
   busy shells → owned busy info-chat;
   body-armor → armor chat;
   **then gold overflow** → gold chat;
   then banword → banword chat;
   then stock structural silent → cash (`myshop_reject_message` override when
   present) → equipped → locked;
   then silk `71049` / ordinary `50200` bag branch;
   success → owned open.
2. **Computation**: sum every request-table listed `price` (no stock-row
   resolution required yet) plus the host's current live gold. If
   `liveGold + Σ(prices) > math.MaxInt32`, emit one self-only `CHAT_TYPE_INFO`
   `You cannot open a private shop because it would exceed 2 Billion Yang.`
   (`vid = 0`, empire `0`). Keep the already-owned `>` / `MaxInt32` threshold;
   do not invent a separate `GOLD_MAX` constant in this slice.
3. **Reject effects**: no `SHOP_SIGN`, no open/busy flag, no bag debit, no
   inventory/quickslot/gold/persistence mutation.
4. **Zero-price / structural stock** stays on the later stock walk (silent
   fail-closed). A zero price does not by itself change the gold sum.
5. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open once GREEN;
   until then this freeze is the source of truth for the next RED.
6. Do **not** invent quest-running open block, `MYSHOP_PRICELIST`, Canada
   banword bypass, bag-missing INFO, already-open second-open empty-sign close,
   polymorph/horse/mount teardown, or tax/empire multipliers.

## Locale / wording note

Keep the already-owned English gold-overflow string. Do not copy oracle source
comments or Korean keys into runtime code.

## Explicit non-goals

- quest-running open block (`PC::IsRunning`)
- `MYSHOP_PRICELIST` / GD price-list packets
- Canada-locale banword bypass / DB banword reload
- bag-missing INFO chat
- already-open second open → empty-sign close (keep current busy INFO)
- polymorph / horse / mount teardown on open
- guest-buy tax / empire `*3`
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Runtime/session: over-cap listed prices + `anti_myshop` stock → gold INFO
   only (not cash / not authored `myshop_reject_message`); no `SHOP_SIGN`.
2. Runtime/session: over-cap listed prices + banword sign → gold INFO only
   (not banword); no open / no bag debit.
3. Runtime/session: over-cap alone → gold INFO; account unchanged.
4. Negative: under-cap gold still hits banword / cash / equipped / locked /
   bag in the owned post-gold order; silk / ordinary success paths unchanged.

## Status

GREEN on `lane/items`: host `CG::MYSHOP` open now sums request-table listed
prices after armor and before banword/stock walk, emitting the owned gold-
overflow INFO when `liveGold + Σ(prices) > math.MaxInt32` without waiting for
cash/equipped/locked row resolution. Quest-running / `MYSHOP_PRICELIST` /
OR-materials stay deferred.
