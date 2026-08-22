# Exchange Start Gold-Carrier Cap Reject Chat — 2026-08-22

## Objective

Fail closed `EXCHANGE START` when either paired side already holds gold at or above the owned signed `PLAYER_POINT_CHANGE` / bootstrap gold carrier max (`1<<31-1`), and make that reject client-visible with locale-shaped info-chat instead of opening a trade shell that can only fail later at gold-overflow finalize.

## Contract to own

1. When the requester's live gold is already `>= exchangeGoldPointChangeCarrierMax` (`1<<31-1`), `START` returns one self-only `CHAT_TYPE_INFO` with `You have more than 2 Billion Yang. You cannot trade.`, emits no exchange frames, creates no pairing/display state, and leaves any open merchant/safebox/refine presentation untouched.
2. When the requester is under the carrier cap but the visible connected target's live gold is already `>= exchangeGoldPointChangeCarrierMax`, `START` returns one self-only `CHAT_TYPE_INFO` with `The player has more than 2 Billion Yang. You cannot trade with him.`, with the same no-pairing / no-exchange-frame contract.
3. Evaluation order on `START` keeps the already-owned busy-window and distance/`ALREADY` gates ahead of this gold-carrier-cap gate. When both sides are over the cap, the requester-side string wins (local-first, matching busy ordering).
4. Spec/QA name these strings beside the owned busy / finalize reject chats; do not invent a new `GC::EXCHANGE` result subheader.

## What this is not yet

- partner-side open player-shop / cube busy-window rejection text
- dual-sided chat for item-id collision / over-template-max / locked-compatible-stack / selected-character / transfer-guard finalization rejects
- changing the already-owned second-accept / commit-time gold-overflow dual-sided chat (`You cannot carry any more gold.` / partner wording)
- restart-restored ground ownership / despawn timers

## TDD and validation (implementation follow-up)

Focused coverage:

- `go test ./internal/minimal -run 'ItemExchangeStartRejects.*Gold|ItemExchangeStartRejectsRequesterGoldCarrier|ItemExchangeStartRejectsPartnerGoldCarrier' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Optionally mirror the same carrier-cap reject on first-side `ACCEPT` if live gold drifts to the cap after shell open.
2. Keep id-collision / restriction finalize reject chat deferred until QA wants those distinguishable from silent fail-closed.
3. Keep partner-side open player-shop / cube busy rejects deferred until those presentation seams exist.

## Status

Contract frozen; implementation / RED not opened in this docs-only commit.
