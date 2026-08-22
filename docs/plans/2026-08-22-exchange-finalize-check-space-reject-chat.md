# Exchange Finalize Check/Space Reject Chat — 2026-08-22

## Objective

Make mutual-accept finalization precondition failures client-visible by emitting dual-sided `CHAT_TYPE_INFO` strings for Check-shaped (displayed item/gold drift) and CheckSpace-shaped (receiver inventory capacity) rejects, instead of failing closed with zero frames.

## Contract to own

1. On second-accept (`AcceptExchange` when the partner had already accepted), when either side's remembered displayed items no longer match live state or either side's remembered displayed gold no longer fits that side's live gold, emit:
   - to the side whose Check failed: `Not enough Yang or the item is not in place.`
   - to the paired partner: `The other player does not have enough Yang or their item is not in place.`
   No accept/`END`/finalize frames, no mutation, shell stays cancellable.
2. On second-accept, when a receiver fails the inventory-capacity half of `exchangeRecipientCanAccept` (cannot place/merge incoming displayed items into carried inventory under loaded templates), emit:
   - to the full receiver: `There isn't enough space in your inventory.`
   - to the paired partner: `The other person has no space left in their inventory.`
   Same no-mutation / still-cancellable contract.
3. Evaluation order on second accept mirrors the already-owned silent gate: requester displayed items → requester displayed gold (`LESS_GOLD` stays the owned requester-gold exception and is unchanged) → accepted-partner displayed items → accepted-partner displayed gold → receiver finalization preconditions. The first matching Check/Space failure wins.
4. `CommitExchangeFinalize` non-busy revalidation uses the same dual-sided Check/Space chat when post-plan drift hits those same classes, rolls back any already-written account/live snapshots from that finalize attempt (already owned), emits no finalize/accept/`END` frames, and leaves the shell cancellable. Busy-window chat stays as already owned and is evaluated before Check/Space.
5. Other second-accept / commit-time receiver precondition failures (item-id collision, over-template-max compatible stack, locked-compatible-stack capacity with no free cell, gold-overflow carrier, selected-character / transfer-guard template restrictions) stay silent/no-frame for this slice.
6. Spec/QA name these dual-sided chats beside the already-owned busy reject chat; do not invent new `GC::EXCHANGE` result subheaders and do not auto-cancel the shell on these rejects.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- auto-cancel / shell teardown on Check/Space reject (legacy Cancel-on-failure stays deferred; bootstrap keeps the shell cancellable like busy rejects)
- chat for gold-overflow / item-id collision / selected-character restriction finalization rejects
- optional authored/template-backed overrides for these strings
- restart-restored ground ownership / despawn timers

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeSecondAcceptRejectsReceiverInventoryCapacity|ItemExchangeSecondAcceptRejectsStaleAcceptedPartner|SharedWorldCommitExchangeFinalizeRejects' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optionally add dual-sided chat for gold-overflow / id-collision / restriction rejects once QA wants those distinguishable from silent fail-closed.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.

## Status

Shipped: second-accept and commit-time Check/Space dual-sided info-chat. Gold-overflow / id-collision / restriction rejects stay silent; player-shop/cube busy rejects stay deferred.
