# Safebox Check-Out In-Memory Mutation — 2026-08-21

## Objective

Freeze the first accepted `SAFEBOX_CHECKOUT` contract for same-session in-memory safebox contents so one stored item can return into carried inventory with client-visible safebox/inventory refresh frames, without inventing password load, durable safebox persistence, mall, money, or item-move yet.

## Contract frozen by this slice

1. `CG::SAFEBOX_CHECKOUT` is accepted only while the selected character already has the bootstrap `/open_safebox` presentation open, is above the zero-HP floor, and is not blocked by an active same-socket exchange shell (close exchange first with self/peer `END` only when the check-out would otherwise succeed; merchant windows stay deferred for this first mutation, matching check-in).
2. `safe_pos` must resolve to exactly one remembered same-session in-memory safebox item inside the currently opened bootstrap capacity (`size * 5` for remembered open size `1..3`). Missing / out-of-range / empty destinations fail closed with no frames.
3. The destination must be a carried inventory cell (`window = INVENTORY`) inside the owned carried range that can accept the whole safebox stack under the loaded template: either an empty cell, or a compatible unlocked unequipped stack that still fits `max_count`. Occupied incompatible / over-max / locked / exchange-displayed destinations fail closed with no frames.
4. On success the runtime removes the item from same-session in-memory safebox state, places/merges it into the destination carried cell while preserving the stored item identity on fresh-cell placement, persists the inventory/quickslot account snapshot, and emits self-only `GC::SAFEBOX_DEL` plus inventory `GC::ITEM_SET` / `GC::ITEM_UPDATE` as appropriate. No gold/money/mall frames are introduced.
5. Spec/QA keep these contents session-scoped: `/close_safebox` still clears only the busy presentation flag; reconnect / process restart / logout still discard remaining in-memory safebox contents until a later persistence slice owns durable safebox state.

## What this is not yet

- password / DB load (`ReqSafeboxLoad`) and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / account-store / DB schema
- `SAFEBOX_ITEM_MOVE`, mall open/checkout
- safebox money / `SAFEBOX_MONEY_CHANGE`
- multi-cell item-size grid packing beyond the bootstrap 1-cell occupancy used by carried inventory today
- NPC / interaction surfaces that open storage windows
- merchant-window auto-close on accepted check-out success

## TDD and validation

Focused coverage (for the later GREEN implementation; this docs freeze intentionally leaves RED deferred):

- `go test ./internal/minimal -run 'SafeboxCheckout|SafeboxCheckin' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Implement the GREEN runtime behind this frozen contract as the next items-lane slice.
2. Keep move / money / password / durable persistence deferred until after the first check-in/out vertical is green.
3. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
