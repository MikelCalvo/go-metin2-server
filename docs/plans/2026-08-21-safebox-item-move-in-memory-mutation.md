# Safebox Item-Move In-Memory Mutation — 2026-08-21

## Objective

Land the first accepted `SAFEBOX_ITEM_MOVE` contract for same-session in-memory safebox contents so one stored stack can relocate (or whole-stack merge) inside the open bootstrap safebox grid with client-visible `SAFEBOX_DEL` / `SAFEBOX_SET` refresh frames, without inventing password load, durable safebox persistence, mall, money, or inventory/quickslot mutation.

## Contract owned by this slice

1. `CG::SAFEBOX_ITEM_MOVE` is accepted only while the selected character already has the bootstrap `/open_safebox` presentation open and is above the zero-HP floor.
2. Source and destination must both name the safebox window (`WindowSafebox`) with distinct cells inside the currently opened bootstrap capacity (`size * 5` for remembered open size `1..3`). Missing / out-of-range / same-cell / non-safebox window requests fail closed with no frames.
3. Source must resolve to exactly one remembered same-session in-memory safebox item. The loaded template for that `vnum` must be valid, must match the stored item `vnum`, and must bound the stored stack count with `max_count`.
4. Requested `count` is whole-stack only for this first mutation: `0` or exactly the live source count. Partial splits (including partial relocate into an empty destination that would need a new item identity) stay fail-closed with no frames.
5. Destination acceptance:
   - empty cell: relocate the whole source stack while preserving item identity, then emit self-only `GC::SAFEBOX_DEL` (source) + `GC::SAFEBOX_SET` (destination);
   - occupied compatible same-`vnum` unlocked unequipped stack whose combined count still fits template `max_count`: merge the whole source count into that destination, clear the source cell, then emit self-only `GC::SAFEBOX_DEL` (source) + `GC::SAFEBOX_SET` (merged destination);
   - occupied incompatible / over-max / locked destinations fail closed with no frames.
6. Success mutates only same-session in-memory safebox contents. Carried inventory, equipment, quickslots, gold, mall, and account persistence stay unchanged. If an active bootstrap exchange shell is open and the move would otherwise succeed, the runtime closes that presentation shell first with self/peer `GC::EXCHANGE END`, then emits the safebox refresh frames; merchant-window auto-close stays deferred.
7. Spec/QA keep these contents session-scoped: `/close_safebox` still clears only the busy presentation flag; reconnect / process restart / logout still discard remaining in-memory safebox contents until a later persistence slice owns durable safebox state.

## What this is not yet

- password / DB load (`ReqSafeboxLoad`) and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / account-store / DB schema
- partial-count splits that create a new safebox item identity in an empty destination
- `SAFEBOX_MONEY_CHANGE` / mall open/checkout
- multi-cell item-size grid packing beyond the bootstrap 1-cell occupancy used by carried inventory today
- NPC / interaction surfaces that open storage windows
- merchant-window auto-close on accepted check-in / check-out / item-move success

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SafeboxItemMove|SafeboxCheckin|SafeboxCheckout' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optional next items-lane seam: merchant-window auto-close on accepted check-in/out/move success (reject path already closes merchant for `anti_safebox` feedback).
2. Keep money / password / durable persistence / partial-split identity allocation deferred.
3. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
