# Exchange Accept Busy-Window Preconditions — 2026-08-20

## Objective

Fail closed for active-shell `EXCHANGE ACCEPT` (including second-accept mutual-accept finalize) when either paired side currently has an open bootstrap merchant window, `/open_safebox` presentation, or refine-dialog presentation, so later-opened busy windows cannot sneak past the already-owned START busy-window trade policy.

## Contract frozen by this slice

1. Requester-side open merchant / safebox / refine presentation rejects `ACCEPT` silently with no accept marker, no finalize frames, no mutation, and a still-cancellable shell.
2. Partner-side open merchant / safebox / refine presentation likewise rejects first-side or second-side `ACCEPT` silently before mutual-accept finalize.
3. Spec/QA wording names those busy-window accept rejects beside the already-owned START busy rejects and other second-accept finalization preconditions.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- richer trade-target eligibility beyond the owned distance + merchant/safebox/refine busy gate
- stronger rollback/audit policy beyond the current fail-closed mutual-accept finalize

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeAcceptRejectsRequesterOpenSafebox|ItemExchangeSecondAcceptRejectsPartnerOpenSafebox|ItemExchangeSecondAcceptRejectsPartnerOpenMerchant' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
2. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
3. Optional authored reject-chat feedback for busy-window accept rejects remains a later presentation seam.
