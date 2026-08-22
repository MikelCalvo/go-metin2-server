# Exchange Finalize Success Chat — 2026-08-22

## Objective

Make successful mutual-accept finalization client-visible by emitting dual-sided `CHAT_TYPE_INFO` success strings that name the trade partner, matching the legacy Done-path feedback, instead of closing the shell with only accept / item / gold / `END` frames.

## Contract to own

1. After a mutual-accept finalize commits (displayed item/gold transfer, source quickslot clears, dual account persistence, and shell teardown), each paired side receives one self-facing `CHAT_TYPE_INFO` with `vid = 0` and message `The trade with <partner_name> has been successful.`, where `<partner_name>` is the other side's normalized live character name.
2. Ordering in each finalize burst: accept marker → inventory / quickslot / gold refresh frames (already owned) → success info-chat → `GC::EXCHANGE END`.
3. Fail-closed finalize paths (busy / Check / Space / gold-overflow / silent Other / persistence rollback) still emit no success chat.
4. Spec/QA name this dual-sided success chat beside the owned reject chats; do not invent a new `GC::EXCHANGE` result subheader.

## What this is not yet

- auto-cancel / shell teardown changes beyond the already-owned mutual-accept `END`
- dual-sided chat for item-id collision / over-template-max / locked-compatible-stack / selected-character / transfer-guard rejects
- partner-side open player-shop / cube busy-window rejection text
- optional authored/template-backed overrides for the success string
- restart-restored ground ownership / despawn timers

## TDD and validation

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeMutualAcceptFinalizesDisplayedTradeAndClosesShell' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optionally add dual-sided chat for id-collision / restriction rejects once QA wants those distinguishable from silent fail-closed.
2. Keep partner-side open player-shop / cube busy-window exchange rejects deferred until those presentation seams exist.
3. Keep ground-item restart durability deferred until operators decide quarantined `0010` exports should drive recovery.

## Status

Shipped: dual-sided mutual-accept success info-chat before shell `END`. Id-collision / restriction rejects stay silent; player-shop/cube busy rejects stay deferred.
