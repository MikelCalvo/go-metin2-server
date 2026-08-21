# Safebox Accepted Mutation Merchant Auto-Close — 2026-08-21

## Objective

Close an active same-socket bootstrap merchant window before the client-visible refresh frames of an accepted open-presentation safebox mutation (`SAFEBOX_CHECKIN`, `SAFEBOX_CHECKOUT`, `SAFEBOX_ITEM_MOVE`), matching the reject-path merchant teardown already owned by `anti_safebox` feedback and the exchange-shell close-on-success path already owned by those mutations.

## Contract owned by this slice

1. When an accepted open-presentation `SAFEBOX_CHECKIN` / `SAFEBOX_CHECKOUT` / `SAFEBOX_ITEM_MOVE` would otherwise succeed and the same socket still has an active bootstrap merchant buy window, the runtime emits self-only `GC::SHOP END` first, clears that merchant presentation, then emits the already-owned mutation refresh frames.
2. Ordering when both presentation shells are active mirrors the reject-path precedent: local `GC::SHOP END` first, then exchange close frame(s), then the safebox/inventory refresh frames.
3. Failed / fail-closed safebox mutation attempts still leave an open merchant window unchanged unless they take the already-owned `anti_safebox` reject-feedback path.
4. Spec/QA name this as presentation-shell hygiene only: no shop buy/sell mutation, no durable safebox persistence, no password/load/money/mall broadening.

## What this is not yet

- password / DB load and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / money / mall
- partial-count safebox splits that allocate a new item identity
- partner-side open player-shop / cube busy-window exchange rejects

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SafeboxCheckinClosesActiveMerchant|SafeboxCheckoutClosesActiveMerchant|SafeboxItemMoveClosesActiveMerchant' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep money / password / durable persistence / partial-split identity allocation deferred.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
