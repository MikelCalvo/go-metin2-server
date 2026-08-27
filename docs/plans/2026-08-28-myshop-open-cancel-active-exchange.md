# MYSHOP open cancels active exchange — 2026-08-28

## Objective

Close the remaining host-only `CG::MYSHOP` open / exchange interaction gap after
already-open second-open empty-sign close: when an accepted open would otherwise
succeed, cancel any active same-socket bootstrap exchange shell (self + peer
`GC::EXCHANGE END`) before emitting the owned bag refresh / live `SHOP_SIGN`
burst, matching the external `OpenMyShop` `m_pkExchange->Cancel()` step without
inventing quest-running open blocks, `MYSHOP_PRICELIST`, bag-missing INFO,
polymorph/mount teardown, or tax/empire multipliers.

## Why this exists

Bootstrap currently folds an open exchange into the shared busy-shell gate and
returns `You cannot trade while another trade window is open.` The TMP4 oracle
instead cancels the exchange on the accepted open success path and continues
into `SHOP_SIGN` / CreatePCShop. Manual QA therefore cannot open a private shop
while a trade window is open without first cancelling, and the busy INFO does
not match oracle. Track C prefers this client-visible cancel-then-open companion
over inventing DB pricelist rematerialize, quest-running, OR-materials, or
binary cube headers.

## Contract to freeze (before RED)

1. **Busy-shell gate** on the accepted decode path no longer treats an active
   same-socket bootstrap exchange as a busy reject for host `CG::MYSHOP` open.
   Merchant / safebox / refine / cube busy shells keep the owned requester busy
   INFO string and ordering.
2. **Cancel position**: only on the accepted open **success** paths (silk
   `71049` bag-less success and ordinary `50200` consume success), after all
   reject gates have passed and after any ordinary bag debit/persist, cancel the
   active exchange shell (if any) before the live `SHOP_SIGN` frame:
   - self receives one `GC::EXCHANGE END`
   - paired peer receives one queued `GC::EXCHANGE END`
   - in-memory pairing/display/accept state is cleared with no inventory /
     equipment / quickslot / gold / ground / exchange trade mutation from the
     cancel itself
3. **Frame ordering on success**:
   - ordinary bag path: bag refresh frames (`ITEM_UPDATE`/`ITEM_DEL` + emptied-cell
     `QUICKSLOT_DEL` when needed) → exchange `END` (when a shell was open) →
     live `SHOP_SIGN` (+ peer around-broadcast as already owned)
   - silk path: exchange `END` (when a shell was open) → live `SHOP_SIGN`
     (+ peer around-broadcast); no bag frames
4. **Reject paths leave exchange open**: armor / gold / banword / cash /
   equipped / locked / structural silent / missing-bag / other busy shells do
   **not** cancel an open exchange. Already-open second-open empty-sign close
   stays unchanged and does not invent exchange cancel beyond clearing the shop.
5. Spec/QA/packet-matrix/roadmap name this beside owned MYSHOP open once GREEN;
   until then this freeze is the source of truth for the next RED.
6. Do **not** invent quest-running open block, `MYSHOP_PRICELIST`, Canada
   banword bypass, bag-missing INFO, polymorph/horse/mount teardown, or
   tax/empire multipliers.

## Locale / wording note

No new English reject string. Success reuses the already-owned exchange `END`
companion and live `SHOP_SIGN`. Do not copy oracle source comments or Korean
keys into runtime code.

## Explicit non-goals

- quest-running open block (`PC::IsRunning`)
- `MYSHOP_PRICELIST` / GD price-list packets
- Canada-locale banword bypass / DB banword reload
- bag-missing INFO chat
- polymorph / horse / mount teardown on open
- guest-buy tax / empire `*3`
- mall / TMP4 CG `SAFEBOX_MONEY` / OR-materials / binary cube headers
- changing guest browse / exchange START busy rejects for an already-open MYSHOP

## Proof shape

1. Runtime/session: paired exchange open → accepted `CG::MYSHOP` (ordinary bag)
   → bag refresh → self `EXCHANGE END` → live `SHOP_SIGN`; peer receives queued
   `EXCHANGE END` then around-broadcast live `SHOP_SIGN`; shop open flag set;
   bag consumed / persisted.
2. Runtime/session: paired exchange open → accepted silk-bag open → self
   `EXCHANGE END` → live `SHOP_SIGN` only (no bag debit).
3. Negative: paired exchange open + armor / gold / banword / cash / equipped /
   locked / missing-bag reject → reject feedback as owned; exchange shell stays
   open (still cancellable); no `SHOP_SIGN`.
4. Negative: merchant / safebox / refine / cube busy still emit busy INFO and
   leave the shop closed (regression).

## Status

GREEN on `lane/items`: accepted open cancels an active exchange before
`SHOP_SIGN` (ordinary bag and silk paths); reject/busy paths leave the shell
open; merchant/safebox/refine/cube busy rejects stay unchanged.
