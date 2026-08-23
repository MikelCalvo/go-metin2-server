# Safebox Money Busy-Shell Auto-Close — 2026-08-23

## Objective

Close an active same-socket bootstrap merchant window and/or exchange shell before the client-visible refresh frames of an accepted open-presentation warehouse-money mutation (`/safebox_money_save` / `/safebox_money_withdraw`), matching the already-owned check-in/out/item-move success teardown ordering so live gold cannot drift under an open trade/merchant presentation.

## Why docs-first

Durable warehouse money already mutates carried gold while the safebox presentation stays open. Check-in / check-out / item-move success already prepend `GC::SHOP END` then exchange `END` before refresh frames, but the money slash path currently emits only gold `PLAYER_POINT_CHANGE` + `SAFEBOX_MONEY_CHANGE` and leaves those shells open. That lets exchange accept-time / commit-time gold revalidation see mid-trade gold drift while the shell still looks active. Opening RED without freezing ordering and fail-closed reject behavior would invent policy.

## Contract to freeze (before RED)

1. Scope: only successful `/safebox_money_save` / `/safebox_money_withdraw` while the same-socket safebox presentation is already open and the mutation would otherwise succeed under the owned amount / gold / overflow / death-floor / persist guards.
2. When an active bootstrap merchant buy window is open, emit self-only `GC::SHOP END` first and clear that merchant presentation before the money refresh frames.
3. When an active bootstrap exchange shell is open, emit self/peer `GC::EXCHANGE END` and clear in-memory exchange display/accept state before the money refresh frames.
4. Ordering when both shells are active mirrors check-in/out/move: local `GC::SHOP END`, then exchange close frame(s), then gold `PLAYER_POINT_CHANGE` + `SAFEBOX_MONEY_CHANGE`.
5. Failed / fail-closed money attempts (closed presentation, pending-only, insufficient gold, overflow, malformed amount, death-floor, persist rollback) leave merchant/exchange shells unchanged and emit no close frames.
6. Spec/QA name this as presentation-shell hygiene beside the owned money mutation; mall / TMP4 CG `SAFEBOX_MONEY` / player-shop/cube busy rejects stay deferred.

## Locale / wording note

No new info-chat strings. Close frames reuse the already-owned `SHOP END` / `EXCHANGE END` families.

## What this is not yet

- mall open/checkout / `MALL_*` runtime emission
- TMP4 CG `SAFEBOX_MONEY` request header
- client `SAFEBOX_CHANGE_PASSWORD` / DB answer frames
- partner-side open player-shop / cube exchange busy rejects
- authored reject-chat text for insufficient gold / warehouse overflow

## TDD shape after the freeze lands

1. Runtime/session: open exchange shell → `/open_safebox` → `/safebox_money_save` emits self `EXCHANGE END` then gold/`SAFEBOX_MONEY_CHANGE`; peer receives queued `END`; durable money + carried gold mutate; shell is gone for later `ACCEPT`.
2. Runtime/session: open merchant → `/open_safebox` → `/safebox_money_withdraw` emits `SHOP END` then gold/`SAFEBOX_MONEY_CHANGE`; later stale `SHOP BUY` fails closed until reopen.
3. Negative: closed/insufficient money attempts leave open merchant/exchange shells untouched.

## Status

Implemented on `lane/items`: successful `/safebox_money_save` / `/safebox_money_withdraw` prepend self-only `GC::SHOP END` and/or self/peer `GC::EXCHANGE END` (SHOP before exchange when both are active) before gold `PLAYER_POINT_CHANGE` + `SAFEBOX_MONEY_CHANGE`; fail-closed money rejects leave merchant/exchange shells untouched. Mall / TMP4 CG `SAFEBOX_MONEY` / player-shop/cube busy rejects stay deferred.
