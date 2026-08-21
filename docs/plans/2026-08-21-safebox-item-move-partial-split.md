# Safebox Item-Move Partial Split — 2026-08-21

## Objective

Accept partial-count `SAFEBOX_ITEM_MOVE` while `/open_safebox` is already open so a same-session in-memory safebox stack can split into an empty destination cell (new item identity) or partially merge into a compatible same-`vnum` destination under template `max_count`, without inventing password load, durable safebox persistence, money, or mall.

## Contract owned by this slice

1. `CG::SAFEBOX_ITEM_MOVE` remains accepted only while the selected character already has the bootstrap `/open_safebox` presentation open and is above the zero-HP floor.
2. Source and destination must both name the safebox window with distinct in-range cells. Missing / out-of-range / same-cell / non-safebox windows stay fail-closed.
3. Requested `count`:
   - `0` or exactly the live source count → keep the already-owned whole-stack relocate / compatible-merge path (`SAFEBOX_DEL` source + `SAFEBOX_SET` destination).
   - `1..source_count-1` → partial path:
     - empty destination: decrement source count in place, allocate a new item identity for the destination stack (max ID across live inventory + equipment + remembered safebox cells, then `+ 1`), place the split count there, emit self-only `SAFEBOX_SET` (source remainder) + `SAFEBOX_SET` (destination split);
     - occupied compatible unlocked unequipped same-`vnum` destination whose combined count still fits template `max_count`: decrement source, add the requested count onto the destination identity, emit self-only `SAFEBOX_SET` (source remainder) + `SAFEBOX_SET` (merged destination);
     - occupied incompatible / over-max / locked destinations, zero/oversize counts other than whole-stack `0`, and ID-allocation overflow stay fail-closed with no frames.
4. Success still mutates only same-session in-memory safebox contents. Carried inventory, equipment, quickslots, gold, mall, and account persistence stay unchanged. Active merchant / exchange presentation shells still auto-close before the refresh frames when the move would otherwise succeed.
5. Spec/QA name the partial-split / partial-merge path beside the owned whole-stack path; password/load/money/durable persistence stay deferred.

## What this is not yet

- password / DB load and `SAFEBOX_WRONG_PASSWORD`
- durable safebox item persistence / money / mall
- partner-side open player-shop / cube busy-window exchange rejects
- multi-cell item-size grid packing beyond bootstrap 1-cell occupancy

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SafeboxItemMove' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep money / password / durable persistence deferred.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
