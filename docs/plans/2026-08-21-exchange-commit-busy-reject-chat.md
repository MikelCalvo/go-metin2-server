# Exchange Commit-Time Busy Reject Chat — 2026-08-21

## Objective

Make commit-time busy-window drift after a mutual-accept finalize plan is built client-visible by emitting the same self-only info-chat strings already owned by exchange `START` / `ACCEPT`, instead of failing closed with zero frames.

## Contract frozen by this slice

1. When `CommitExchangeFinalize` finds the commit requester (`plan.OriginID`, the second accepter) has an open merchant / safebox / refine presentation, it returns one self-only `CHAT_TYPE_INFO` (`You cannot trade while another trade window is open.`), applies no live shared-world trade mutation, emits no finalize/accept/`END` frames, and leaves the shell cancellable.
2. When only the paired partner has one of those busy presentations open, it returns one self-only `CHAT_TYPE_INFO` (`That player cannot trade right now.`) with the same no-mutation / still-cancellable contract.
3. When both sides are busy, the commit-requester busy text wins, matching the local-first `START` / `ACCEPT` busy ordering.
4. Non-busy commit-time revalidation failures (displayed item/gold drift, receiver finalization precondition drift) stay silent/no-frame.
5. Spec/QA name commit-time busy reject chat beside the already-owned ACCEPT busy reject chat; the factory rolls back any already-written account/live snapshots from that finalize attempt before returning the busy chat frames.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- optional authored/template-backed reject-chat overrides for busy-window rejects
- richer trade-target eligibility beyond the owned distance + merchant/safebox/refine busy gate + transfer teardown
- restart-restored ground ownership / despawn timers

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SharedWorldCommitExchangeFinalizeRejectsBusyWindowOpenedAfterAcceptPlan|SharedWorldAcceptExchangeRejectsOpenRefineWindow' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
2. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
3. Keep accepted safebox password/load/placement/money deferred.
