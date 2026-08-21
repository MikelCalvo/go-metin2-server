# Exchange Finalize Commit-Time Revalidation — 2026-08-20

## Objective

Fail closed at mutual-accept commit/apply time when either paired side opens a merchant / safebox / refine busy presentation, or live displayed item/gold / receiver finalization preconditions drift, between `AcceptExchange` plan creation and `CommitExchangeFinalize`.

## Contract frozen by this slice

1. `CommitExchangeFinalize` re-checks either-side open merchant / safebox / refine busy presentations under the shared-world lock before applying live shared-world character updates, enqueueing finalize frames, or clearing the shell.
2. The same commit gate revalidates both sides' remembered displayed items/gold against current shared-world live characters and loaded templates, and re-runs the already-owned receiver finalization preconditions against current live characters.
3. On any commit-time recheck failure: no durable trade mutation remains (account/live rollbacks already owned by `applyExchangeFinalize` stay in force), no finalize/accept/`END` frames are emitted, and the shell stays cancellable.
4. Spec/QA name commit-time busy / display / precondition revalidation beside the already-owned `AcceptExchange` busy-window and second-accept precondition guards.
5. Shared-world unit coverage freezes at least one busy-window drift case: second-accept plan is built successfully, then a paired side opens safebox (or merchant/refine) before commit, and `CommitExchangeFinalize` fails closed.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- optional authored/template-backed reject-chat overrides beyond the fixed START busy strings now also used by ACCEPT and commit-time busy reject chat
- richer rollback/audit policy beyond the current fail-closed mutual-accept finalize
- ground-item restart durability

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'SharedWorldCommitExchangeFinalizeRejectsBusyWindowOpenedAfterAcceptPlan|SharedWorldAcceptExchangeRejectsOpenRefineWindow' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
2. ACCEPT and commit-time busy-window reject info-chat now reuse the START requester/partner strings; optional authored/template-backed overrides remain later presentation seams.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.
