# Exchange Transfer / Warp Rebootstrap Teardown — 2026-08-20

## Objective

Close an active bootstrap exchange shell before exact-position transfer / warp rebootstrap delivers its self burst, including same-map in-range destinations that would still satisfy the walk-away distance gate, so mutual-accept finalize cannot run after relocation.

## Contract frozen by this slice

1. Paired exchange + exact-position transfer / warp rebootstrap prepends self `GC::EXCHANGE END` before the normal transfer bootstrap / origin visibility frames and queues peer `GC::EXCHANGE END`.
2. Same-map in-range destinations still tear the shell down; the runtime does not rely only on walk-away distance.
3. Mid-accept shells (one side already accepted) tear down the same way; later `ACCEPT` fails closed with no finalize frames.
4. Inventory, equipment, quickslots, gold, ground handles, and exchange trade mutation stay unchanged by the teardown itself; destination map/x/y persistence follows the already-owned transfer contract.
5. When an open merchant window is also closed by that same transfer, already-owned merchant `GC::SHOP END` precedes exchange `END`.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- richer trade-target eligibility beyond the owned distance + merchant/safebox/refine busy gate + transfer teardown
- stronger rollback/audit policy beyond the current fail-closed mutual-accept finalize
- optional authored/template-backed reject-chat overrides beyond the fixed START busy strings now also used by ACCEPT and commit-time busy reject chat

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeTransferTriggerClosesShell|ItemExchangeSameMapInRangeTransferClosesShell|ItemExchangeTransferClosesAcceptedShellBeforeSecondAccept' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
2. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
3. ACCEPT and commit-time busy-window reject info-chat now reuse the START requester/partner strings; optional authored/template-backed overrides remain later presentation seams.
