# Exchange Finalize Gold-Overflow Reject Chat — 2026-08-22

## Objective

Make mutual-accept finalization receiver gold-overflow precondition failures client-visible by emitting dual-sided `CHAT_TYPE_INFO` strings, matching the already-owned Check/Space reject-chat pattern, instead of failing closed with zero frames.

## Contract to own

1. On second-accept (`AcceptExchange` when the partner had already accepted), when a receiver fails because accepting the displayed incoming gold would overflow the signed `PLAYER_POINT_CHANGE` / bootstrap gold carrier (`You cannot carry any more gold.` class), emit:
   - to the overflow receiver: `You cannot carry any more gold.`
   - to the paired partner: `The other person cannot carry any more gold.`
   No accept/`END`/finalize frames, no mutation, shell stays cancellable.
2. `CommitExchangeFinalize` non-busy revalidation uses the same dual-sided gold-overflow chat when post-plan receiver gold overflow hits, rolls back any already-written account/live snapshots from that finalize attempt (already owned), emits no finalize/accept/`END` frames, and leaves the shell cancellable. Busy-window chat stays as already owned and is evaluated before Check/Space/gold-overflow. Check/Space chat stays as already owned and is evaluated before gold-overflow when both could apply.
3. Evaluation order on second accept / commit remains: busy windows → Check-shaped displayed item/gold drift → Space (inventory capacity) → other receiver preconditions. Gold-overflow is the first owned chat-emitting member of that remaining "other" class; item-id collision / over-template-max / locked-compatible-stack / selected-character / transfer-guard rejects stay silent/no-frame for this slice.
4. Reuse the already-owned quest-reward overflow self string `You cannot carry any more gold.` for the overflow receiver so QA sees one consistent gold-cap message. Partner wording mirrors the SpaceOther pattern.
5. Spec/QA name this dual-sided chat beside the already-owned Check/Space / busy reject chats; do not invent new `GC::EXCHANGE` result subheaders and do not auto-cancel the shell on these rejects.

## What this is not yet

- dual-sided chat for item-id collision / over-template-max / locked-compatible-stack / selected-character / transfer-guard finalization rejects
- partner-side open player-shop / cube busy-window rejection text
- auto-cancel / shell teardown on gold-overflow reject
- optional authored/template-backed overrides for these strings
- restart-restored ground ownership / despawn timers

## TDD and validation (implementation follow-up)

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeSecondAcceptRejectsReceiverGoldOverflow|SharedWorldCommitExchangeFinalizeRejects' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optionally add dual-sided chat for id-collision / restriction rejects once QA wants those distinguishable from silent fail-closed.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.

## Status

Docs/spec contract freeze. RED/GREEN implementation follows in the next items-lane step.
