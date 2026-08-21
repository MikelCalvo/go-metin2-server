# Safebox Check-In In-Memory Mutation — 2026-08-21

## Objective

Land the first accepted `SAFEBOX_CHECKIN` contract for an already-open bootstrap safebox presentation so one carried inventory item can move into an in-memory same-session safebox slot with client-visible inventory/safebox refresh frames, without inventing password load, durable safebox persistence, mall, or money yet.

## Contract owned by this slice

1. `CG::SAFEBOX_CHECKIN` is accepted only while the selected character already has the bootstrap `/open_safebox` presentation open, is above the zero-HP floor, and is not blocked by an exchange-displayed carried cell (close exchange first with self/peer `END` only when the check-in would otherwise succeed; merchant windows stay deferred for this first mutation).
2. The request must name inventory-window carried cell metadata that resolves to exactly one unlocked, unequipped, well-formed live item whose loaded template is valid, matches `vnum`, bounds the live count with `max_count`, and does **not** author `anti_safebox`. `anti_safebox` keeps the already-owned authored `safebox_reject_message` reject (or silent fail-closed when that text is omitted).
3. `safe_pos` must be empty and inside the currently opened bootstrap capacity (`size * 5` cells for remembered open size `1..3`, matching the oracle 5-wide grid). Occupied / out-of-range destinations fail closed with no frames.
4. On success the runtime removes the carried item from inventory, syncs source item quickslots with the already-owned removal path (`GC::QUICKSLOT_DEL` when the cell is fully removed), stores the item in the same-session in-memory safebox slot, persists the inventory/quickslot account snapshot, and emits self-only `GC::ITEM_DEL` plus `GC::SAFEBOX_SET` for that safebox cell. No gold/money/mall frames are introduced.
5. Spec/QA name this as session-scoped in-memory safebox contents: `/close_safebox` clears only the busy presentation flag; reopening with `/open_safebox` re-emits `SAFEBOX_SIZE` and re-emits remembered in-memory `SAFEBOX_SET` rows for the still-open session. Reconnect / process restart / logout still lose those contents until a later persistence slice owns durable safebox state.

## What this is not yet

- password / DB load (`ReqSafeboxLoad`) and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / account-store / DB schema
- `SAFEBOX_CHECKOUT`, `SAFEBOX_ITEM_MOVE`, mall open/checkout
- safebox money / `SAFEBOX_MONEY_CHANGE`
- multi-cell item-size grid packing beyond the bootstrap 1-cell occupancy used by carried inventory today
- NPC / interaction surfaces that open storage windows
- merchant-window auto-close on accepted check-in success

## TDD and validation

Focused coverage:

- `go test ./internal/player -run 'SafeboxCheckin' -count=1`
- `go test ./internal/minimal -run 'SafeboxCheckin|OpenSafebox|AntiSafebox' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Implement the first accepted `SAFEBOX_CHECKOUT` from same-session in-memory contents back into carried inventory (`docs/plans/2026-08-21-safebox-checkout-in-memory-mutation.md`).
2. Keep move / money / password / durable persistence deferred until after the first check-in/out vertical is green.
3. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
