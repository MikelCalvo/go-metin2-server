# Exchange Accept Busy-Window Reject Chat — 2026-08-21

## Objective

Make active-shell `EXCHANGE ACCEPT` busy-window rejects client-visible by emitting the same self-only info-chat strings already owned by exchange `START`, instead of failing closed with zero frames.

## Contract frozen by this slice

1. Requester-side open merchant / safebox / refine presentation rejects `ACCEPT` with one self-only `CHAT_TYPE_INFO` (`You cannot trade while another trade window is open.`), no accept marker, no finalize frames, no mutation, and a still-cancellable shell.
2. Partner-side open merchant / safebox / refine presentation rejects first-side or second-side `ACCEPT` with one self-only `CHAT_TYPE_INFO` (`That player cannot trade right now.`), with the same no-accept-marker / no-mutation / still-cancellable contract.
3. When both sides are busy, the requester busy text wins, matching the local-first `START` busy ordering.
4. Shared-world `AcceptExchange` unit coverage for open refine mirrors the same requester/partner chat frames.
5. Commit-time busy drift after a second-accept finalize plan is built now emits the same self-only START/ACCEPT busy info-chat; non-busy commit-time revalidation failures stay silent/no-frame.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- optional authored/template-backed reject-chat overrides for busy-window rejects
- richer trade-target eligibility beyond the owned distance + merchant/safebox/refine busy gate + transfer teardown
- commit-time busy-window reject chat after `AcceptExchange` plan creation (now owned by `2026-08-21-exchange-commit-busy-reject-chat.md`)

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeAcceptRejectsRequesterOpenSafebox|ItemExchangeAcceptRejectsRequesterOpenMerchant|ItemExchangeAcceptRejectsPartnerOpenSafebox|ItemExchangeAcceptRejectsPartnerOpenMerchant|ItemExchangeSecondAcceptRejectsPartnerOpenSafebox|ItemExchangeSecondAcceptRejectsPartnerOpenMerchant|SharedWorldAcceptExchangeRejectsOpenRefineWindow' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
2. Commit-time busy reject chat is now owned beside ACCEPT busy reject chat; optional authored/template-backed overrides remain deferred.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
