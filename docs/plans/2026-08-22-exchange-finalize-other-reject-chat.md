# Exchange Finalize Other Reject Chat — 2026-08-22

## Objective

Make the remaining second-accept / commit-time receiver finalization precondition failures client-visible by emitting dual-sided `CHAT_TYPE_INFO` `"Unknown error"` for the already-classified `exchangeRecipientRejectOther` bucket (item-id collision, over-template-max compatible stack, locked-compatible-stack capacity with no free cell, selected-character / transfer-guard template restrictions, and invalid receiver snapshots), instead of failing closed with zero frames.

## Contract owned by this slice

1. On second-accept (`AcceptExchange` when the partner had already accepted), when a receiver fails with `exchangeRecipientRejectOther` after the already-owned busy / gold-carrier-cap / Check / Space / gold-overflow gates, emit:
   - to both paired sides: one self-facing `CHAT_TYPE_INFO` with message `Unknown error` (`vid = 0`)
   - no accept/`END`/finalize frames, no inventory/equipment/quickslot/gold/persistence mutation, shell stays cancellable
2. Evaluation order on second accept / commit remains: busy windows → gold-carrier-cap → Check-shaped displayed item/gold drift → Space (inventory capacity) → gold-overflow → Other. The first matching chat-emitting class wins; Other is the last owned chat class in that chain.
3. `CommitExchangeFinalize` non-busy revalidation uses the same dual-sided `Unknown error` chat when post-plan receiver drift hits Other, rolls back any already-written account/live snapshots from that finalize attempt (already owned), emits no finalize/accept/`END` frames, and leaves the shell cancellable. Busy / gold-carrier-cap / Check / Space / gold-overflow chats stay as already owned and are evaluated before Other.
4. Spec/QA name this dual-sided Other chat beside the owned Check / Space / gold-overflow / success chats; do not invent a new `GC::EXCHANGE` result subheader and do not auto-cancel the shell on these rejects (bootstrap keeps the shell cancellable like the other owned finalize reject chats).
5. Do not invent per-reason English strings for id-collision vs restriction vs over-max in this slice. The external behavior oracle uses literal `"Unknown error"` for non-Check/Space mutual-accept abort paths that are not the ordinary Check/Space chats; this slice reuses that same catch-all wording for the remaining Other bucket.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- auto-cancel / shell teardown on Other reject (legacy Cancel-on-failure stays deferred; bootstrap keeps the shell cancellable)
- distinct per-reason Other chat strings beyond `Unknown error`
- optional authored/template-backed overrides for these strings
- durable safebox persistence / password / money

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeSecondAcceptRejectsReceiverEquipmentIDCollision|ItemExchangeSecondAcceptRejectsReceiverInventoryIDCollision|ItemExchangeSecondAcceptRejectsReceiverSelectedCharacterRestriction|ItemExchangeSecondAcceptRejectsReceiverLockedCompatible|ItemExchangeSecondAcceptRejectsReceiverOverTemplate|SharedWorldCommitExchangeFinalizeRejects' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
2. Keep durable safebox persistence / password / money deferred behind a separate store/schema contract freeze.
3. Optional later: replace catch-all `Unknown error` with distinguishable per-reason strings only when QA has explicit locale evidence for each case.

## Status

Shipped: second-accept and commit-time `exchangeRecipientRejectOther` emit dual-sided `CHAT_TYPE_INFO` `Unknown error` while leaving the shell cancellable and performing no trade mutation. Player-shop/cube busy rejects and durable safebox persistence stay deferred.
