# MYSHOP already-open second open → empty-sign close — 2026-08-27

## Objective

Close the remaining host-only `CG::MYSHOP` open gap after gold-overflow gate
reorder: when the host already has an open private shop, a second accepted
decode `CG::MYSHOP` should clear/close with the owned empty-sign companion
instead of emitting busy INFO, matching the external `OpenMyShop` already-open
branch without inventing quest-running open blocks, `MYSHOP_PRICELIST`,
bag-missing INFO, or tax/empire multipliers.

## Why this exists

Bootstrap currently folds already-open MYSHOP into the shared busy-shell gate
and returns `You cannot trade while another trade window is open.` The TMP4
oracle instead calls `CloseMyShop()` (empty-sign clear) and returns. Track C
prefers this client-visible close companion over inventing DB pricelist
rematerialize, quest-running, OR-materials, or binary cube headers.

## Contract to freeze (before RED)

1. **Gate position** on the accepted decode path:
   death-floor / no-selected / no-shared → silent;
   empty sign / zero count → silent (does **not** close an already-open shop);
   busy shells **other than** already-open MYSHOP → owned busy info-chat;
   body-armor → armor chat (wins even when already open);
   **then already-open MYSHOP** → empty-sign clear/close;
   then gold overflow → gold chat;
   then banword → banword chat;
   then stock structural silent → cash (`myshop_reject_message` override when
   present) → equipped → locked;
   then silk `71049` / ordinary `50200` bag branch;
   success → owned open.
2. **Already-open close effects**: reuse the owned empty-sign clear/close
   companion already used by lifecycle `/phase_select` / `/quit` / `/logout`,
   practice-mob floor, transfer/warp, and lab `/close_myshop`:
   - clear the same-socket open/busy flag;
   - emit one empty-sign `GC::SHOP_SIGN` to self;
   - peer around-broadcast of that empty sign as already owned;
   - clear any guest browse with one guest-only `GC::SHOP END` as already owned;
   - no bag refund, no inventory/quickslot/gold/persistence mutation from this
     second-open path;
   - no busy INFO chat.
3. **Other busy shells** (merchant / safebox / refine / cube / exchange) keep
   the owned busy INFO string and ordering.
4. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open once GREEN;
   until then this freeze is the source of truth for the next RED.
5. Do **not** invent quest-running open block, `MYSHOP_PRICELIST`, Canada
   banword bypass, bag-missing INFO, polymorph/horse/mount teardown, or
   tax/empire multipliers.

## Locale / wording note

No new English reject string. The close path reuses the already-owned empty
`GC::SHOP_SIGN` companion. Do not copy oracle source comments or Korean keys
into runtime code.

## Explicit non-goals

- quest-running open block (`PC::IsRunning`)
- `MYSHOP_PRICELIST` / GD price-list packets
- Canada-locale banword bypass / DB banword reload
- bag-missing INFO chat
- bag refund on second-open close
- polymorph / horse / mount teardown on open
- guest-buy tax / empire `*3`
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers

## Proof shape

1. Runtime/session: open successfully → second `CG::MYSHOP` with valid
   non-empty sign + stock → empty-sign clear (self + peer around-broadcast);
   no busy INFO; no bag refund; open flag cleared.
2. Runtime/session: already open + worn body armor + second open → armor INFO
   only (shop stays open).
3. Runtime/session: already open + empty-sign / zero-count second packet →
   silent; shop stays open.
4. Negative: under already-closed path, armor / gold / banword / cash /
   equipped / locked / bag order stays as owned after this gate; silk /
   ordinary success paths unchanged.

## Status

Docs/spec freeze only on `lane/items`. Runtime RED/GREEN intentionally deferred
to the next implementation slice.
